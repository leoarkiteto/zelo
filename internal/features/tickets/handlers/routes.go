package handlers

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// RegisterRoutes registers the tickets routes on mux. Member routes allow any
// active condominium member (owner, tenant, syndic); inbox/reply/close are
// syndic-only per contracts/http-routes.md.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	h := &Handler{deps: deps}

	memberOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleOwner, model.RoleTenant, model.RoleSyndic)
	syndicOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleSyndic)

	mux.Handle("GET /tickets", middleware.RequireAuth(memberOnly(http.HandlerFunc(h.ticketsGET))))
	mux.Handle("GET /tickets/new", middleware.RequireAuth(memberOnly(http.HandlerFunc(h.ticketsNewGET))))
	mux.Handle("POST /tickets", middleware.RequireAuth(memberOnly(http.HandlerFunc(h.ticketsCreatePOST))))
	mux.Handle("GET /tickets/{id}", middleware.RequireAuth(memberOnly(http.HandlerFunc(h.ticketDetailGET))))
	mux.Handle("GET /tickets/inbox", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.inboxGET))))
	mux.Handle("POST /tickets/{id}/reply", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.ticketReplyPOST))))
	mux.Handle("POST /tickets/{id}/close", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.ticketClosePOST))))
}
