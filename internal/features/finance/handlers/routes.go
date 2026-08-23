package handlers

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// RegisterRoutes registers the finance routes on mux. Management routes are
// syndic-only; resident routes allow any active condominium member.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	h := &Handler{deps: deps}

	syndicOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleSyndic)
	memberOnly := middleware.RequireRole(deps.Roles, deps.Audit, model.RoleOwner, model.RoleTenant, model.RoleSyndic)

	mux.Handle("GET /finance", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.financeGET))))
	mux.Handle("GET /finance/new", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.financeNewGET))))
	mux.Handle("POST /finance", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.financeCreatePOST))))
	mux.Handle("GET /finance/{id}/edit", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.financeEditGET))))
	mux.Handle("POST /finance/{id}/edit", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.financeEditPOST))))
	mux.Handle("GET /finance/{id}/settle", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.financeSettleGET))))
	mux.Handle("POST /finance/{id}/settle", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.financeSettlePOST))))
	mux.Handle("GET /finance/{id}/cancel", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.financeCancelGET))))
	mux.Handle("POST /finance/{id}/cancel", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.financeCancelPOST))))
	mux.Handle("GET /finance/{id}/receipt", middleware.RequireAuth(syndicOnly(http.HandlerFunc(h.financeReceiptGET))))
	mux.Handle("GET /finance/my-charges", middleware.RequireAuth(memberOnly(http.HandlerFunc(h.myChargesGET))))
	mux.Handle("GET /finance/health", middleware.RequireAuth(memberOnly(http.HandlerFunc(h.healthGET))))
}
