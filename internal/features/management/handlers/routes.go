package handlers

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// RegisterRoutes registers the syndic-only management routes on mux.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	h := &Handler{deps: deps}

	syndicOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleSyndic)

	mux.Handle("GET /invitations", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.invitationsGET))))
	mux.Handle("POST /invitations", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.invitationsPOST))))
	mux.Handle("POST /invitations/{id}/revoke", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.invitationsRevoke))))
	mux.Handle("GET /roles", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.rolesGET))))
	mux.Handle("POST /roles/assign", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.rolesAssign))))
	mux.Handle("POST /roles/revoke", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.rolesRevoke))))
}
