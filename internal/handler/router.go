package handler

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/middleware"
	"github.com/leoarkiteto/zelo/internal/model"
)

// NewRouter builds the full HTTP handler with middleware applied.
func NewRouter(deps Dependencies) http.Handler {
	h := &Handler{deps: deps}
	mux := http.NewServeMux()

	// Public routes.
	mux.HandleFunc("GET /register", h.registerGET)
	mux.HandleFunc("POST /register", h.registerPOST)
	mux.HandleFunc("GET /login", h.loginGET)
	mux.HandleFunc("POST /login", h.loginPOST)
	mux.HandleFunc("GET /password/forgot", h.forgotGET)
	mux.HandleFunc("POST /password/forgot", h.forgotPOST)
	mux.HandleFunc("GET /password/reset", h.resetGET)
	mux.HandleFunc("POST /password/reset", h.resetPOST)

	// Authenticated routes.
	mux.Handle("POST /logout", middleware.RequireAuth(http.HandlerFunc(h.logoutPOST)))
	mux.Handle("GET /", middleware.RequireAuth(http.HandlerFunc(h.home)))

	// Role-restricted areas (US3).
	syndicOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleSyndic)
	ownerOrSyndic := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleOwner, model.RoleSyndic)
	tenantOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleTenant)
	memberOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleOwner, model.RoleTenant, model.RoleSyndic)

	mux.Handle("GET /condominium", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.condominium))))
	mux.Handle("GET /unit", middleware.RequireAuth(ownerOrSyndic(http.HandlerFunc(h.unit))))
	mux.Handle("GET /tenancy", middleware.RequireAuth(tenantOnly(http.HandlerFunc(h.tenancy))))

	// Service directory (member access + syndic moderation).
	mux.Handle("GET /directory", middleware.RequireAuth(memberOnly(http.HandlerFunc(h.directoryGET))))
	mux.Handle("GET /directory/new", middleware.RequireAuth(memberOnly(http.HandlerFunc(h.directoryNewGET))))
	mux.Handle("POST /directory", middleware.RequireAuth(memberOnly(http.HandlerFunc(h.directoryCreatePOST))))
	mux.Handle("GET /directory/{id}/edit", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.directoryEditGET))))
	mux.Handle("POST /directory/{id}/edit", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.directoryEditPOST))))
	mux.Handle("GET /directory/{id}/delete", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.directoryDeleteConfirmGET))))
	mux.Handle("POST /directory/{id}/delete", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.directoryDeletePOST))))
	mux.Handle("GET /directory/categories", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.categoriesGET))))
	mux.Handle("POST /directory/categories", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.categoriesCreatePOST))))
	mux.Handle("POST /directory/categories/{id}/rename", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.categoriesRenamePOST))))
	mux.Handle("POST /directory/categories/{id}/deactivate", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.categoriesDeactivatePOST))))

	// Syndic-only management (US4).
	mux.Handle("GET /invitations", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.invitationsGET))))
	mux.Handle("POST /invitations", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.invitationsPOST))))
	mux.Handle("POST /invitations/{id}/revoke", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.invitationsRevoke))))
	mux.Handle("GET /roles", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.rolesGET))))
	mux.Handle("POST /roles/assign", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.rolesAssign))))
	mux.Handle("POST /roles/revoke", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.rolesRevoke))))

	// Static assets.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	var root http.Handler = mux
	root = middleware.Recover(deps.Logger)(root)
	root = middleware.Logging(deps.Logger)(root)
	root = middleware.SecurityHeaders(root)
	root = middleware.WithUser(deps.Sessions, deps.Users)(root)
	root = middleware.CSRF(deps.Sessions)(root)
	return root
}
