package handlers

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// RegisterRoutes registers the service directory routes on mux.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	h := &Handler{deps: deps}

	syndicOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleSyndic)
	memberOnly := middleware.RequireRole(
		deps.Roles,
		deps.Audit,
		model.RoleOwner,
		model.RoleTenant,
		model.RoleSyndic,
	)

	mux.Handle(
		"GET /directory",
		middleware.RequireAuth(memberOnly(http.HandlerFunc(h.directoryGET))),
	)
	mux.Handle(
		"GET /directory/new",
		middleware.RequireAuth(memberOnly(http.HandlerFunc(h.directoryNewGET))),
	)
	mux.Handle(
		"POST /directory",
		middleware.RequireAuth(memberOnly(http.HandlerFunc(h.directoryCreatePOST))),
	)
	mux.Handle(
		"GET /directory/{id}/edit",
		middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.directoryEditGET))),
	)
	mux.Handle(
		"POST /directory/{id}/edit",
		middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.directoryEditPOST))),
	)
	mux.Handle(
		"GET /directory/{id}/delete",
		middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.directoryDeleteConfirmGET))),
	)
	mux.Handle(
		"POST /directory/{id}/delete",
		middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.directoryDeletePOST))),
	)
	mux.Handle(
		"GET /directory/categories",
		middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.categoriesGET))),
	)
	mux.Handle(
		"POST /directory/categories",
		middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.categoriesCreatePOST))),
	)
	mux.Handle(
		"POST /directory/categories/{id}/rename",
		middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.categoriesRenamePOST))),
	)
	mux.Handle(
		"POST /directory/categories/{id}/deactivate",
		middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.categoriesDeactivatePOST))),
	)
}
