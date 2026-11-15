package handlers

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

var syndicRoles = []model.Role{model.RoleSyndic}

// RegisterRoutes registers the invoice, supplier and fragment routes on mux.
// Every route is syndic-only and scoped to the session's condominium.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	h := &Handler{deps: deps}

	syndicOnly := middleware.RequireRole(deps.Roles, deps.Audit, syndicRoles...)
	page := func(fn http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(syndicOnly(fn))
	}
	// Fragments are swapped into a page by htmx: an expired session must not
	// swap the login page into the form.
	fragment := func(fn http.HandlerFunc) http.Handler {
		return h.hxAwareSyndicOnly(fn)
	}

	mux.Handle("GET /invoices", page(h.listGET))
	mux.Handle("GET /invoices/new", page(h.newGET))
	mux.Handle("POST /invoices", page(h.createPOST))
	mux.Handle("GET /invoices/{id}", page(h.viewGET))
	mux.Handle("GET /invoices/{id}/edit", page(h.editGET))
	mux.Handle("POST /invoices/{id}/edit", page(h.editPOST))
	mux.Handle("POST /invoices/{id}/delete", page(h.deletePOST))
	mux.Handle("POST /invoices/{id}/annul", page(h.annulPOST))
	mux.Handle("POST /invoices/{id}/payments", page(h.paymentPOST))
	mux.Handle("POST /invoices/{id}/attachments", page(h.attachmentsPOST))
	mux.Handle("GET /invoices/{id}/attachments/{aid}", page(h.attachmentGET))
	mux.Handle("POST /invoices/{id}/attachments/{aid}/delete", page(h.attachmentDeletePOST))

	mux.Handle("GET /invoices/form/section", fragment(h.sectionGET))
	mux.Handle("POST /invoices/form/lines", fragment(h.linesPOST))
	mux.Handle("POST /invoices/form/preview", fragment(h.previewPOST))

	mux.Handle("GET /suppliers", page(h.suppliersGET))
	mux.Handle("GET /suppliers/new", page(h.supplierNewGET))
	mux.Handle("POST /suppliers", page(h.supplierCreatePOST))
	mux.Handle("GET /suppliers/{id}/edit", page(h.supplierEditGET))
	mux.Handle("POST /suppliers/{id}/edit", page(h.supplierEditPOST))
	mux.Handle("GET /suppliers/search", fragment(h.supplierSearchGET))
	mux.Handle("POST /suppliers/inline", fragment(h.supplierInlinePOST))
}

// hxAwareSyndicOnly guards the HTMX fragment routes. htmx v4 swaps every
// response status except 204/304, so for HX requests an expired session answers
// HX-Redirect to /login and a denied role an empty 403, instead of a 303 login
// page or a plain-text body being swapped into the form. Plain requests keep
// the standard chain.
func (h *Handler) hxAwareSyndicOnly(next http.Handler) http.Handler {
	standard := middleware.RequireAuth(
		middleware.RequireRole(h.deps.Roles, h.deps.Audit, syndicRoles...)(next))
	hxGuard := middleware.RequireRoleWith(h.deps.Roles, h.deps.Audit, middleware.RoleGuardResponders{
		Unauthenticated: func(w http.ResponseWriter, r *http.Request) {
			middleware.HXRedirect(w, "/login")
		},
		Denied: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		},
	}, syndicRoles...)(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "HX-Request")
		if middleware.IsHX(r) {
			hxGuard.ServeHTTP(w, r)
			return
		}
		standard.ServeHTTP(w, r)
	})
}
