package handlers

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// RegisterRoutes registers the home/dashboard routes on mux.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	h := &Handler{deps: deps}

	syndicOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleSyndic)
	ownerOrSyndic := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleOwner, model.RoleSyndic)
	tenantOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleTenant)

	mux.Handle("GET /", middleware.RequireAuth(http.HandlerFunc(h.home)))
	mux.Handle("GET /condominium", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.condominium))))
	mux.Handle("GET /unit", middleware.RequireAuth(ownerOrSyndic(http.HandlerFunc(h.unit))))
	mux.Handle("GET /tenancy", middleware.RequireAuth(tenantOnly(http.HandlerFunc(h.tenancy))))
}
