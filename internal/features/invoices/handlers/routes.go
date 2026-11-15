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
	// A denied role answers an empty 403, which htmx must not swap in.
	hxGuard := middleware.RequireRoleHX(deps.Roles, deps.Audit, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}, syndicRoles...)
	fragment := func(fn http.HandlerFunc) http.Handler {
		return hxGuard(fn)
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
	mux.Handle("GET /suppliers/{id}/selected", fragment(h.supplierSelectedGET))
	mux.Handle("POST /suppliers/inline", fragment(h.supplierInlinePOST))
}
