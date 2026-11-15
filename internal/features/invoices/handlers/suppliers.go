package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
)

const suppliersPath = "/suppliers"

func supplierValues(s domain.Supplier) templates.SupplierValues {
	return templates.SupplierValues{
		ID: s.ID, Name: s.Name, NIF: s.NIF, Country: s.Country, Address: s.Address,
		PostalCode: s.PostalCode, Locality: s.Locality, Email: s.Email, Phone: s.Phone,
	}
}

// supplierFromForm reads the supplier inputs; prefix is "supplier_" for the
// inline form of the invoice page and "" on the supplier pages.
func supplierFromForm(r *http.Request, prefix string) domain.Supplier {
	g := func(f string) string { return strings.TrimSpace(r.FormValue(prefix + f)) }
	return domain.Supplier{
		Name: g("name"), NIF: g("nif"), Country: g("country"), Address: g("address"),
		PostalCode: g("postal_code"), Locality: g("locality"), Email: g("email"), Phone: g("phone"),
	}
}

// suppliersGET lists the suppliers of the condominium, optionally filtered.
func (h *Handler) suppliersGET(w http.ResponseWriter, r *http.Request) {
	_, condo := actor(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	list, err := h.deps.Suppliers.Search(r.Context(), condo, q, 0)
	if err != nil {
		serverError(w, r, "list suppliers", err, "invoices.error.load_failed")
		return
	}
	sh, err := shell(r, suppliersPath)
	if err != nil {
		serverError(w, r, "shell", err, "invoices.error.load_failed")
		return
	}
	httpx.Render(w, r, templates.SupplierListPage(templates.SupplierListData{
		Shell:     sh,
		Locale:    locale(r),
		CSRF:      httpx.CurrentSession(r).CSRFToken,
		Query:     q,
		Suppliers: list,
		Flash:     flashText(r, "suppliers.flash.", "created", "updated"),
	}))
}

func (h *Handler) renderSupplierForm(w http.ResponseWriter, r *http.Request, status int, edit bool, v templates.SupplierValues, errs domain.FieldErrors, msg string) {
	sh, err := shell(r, suppliersPath)
	if err != nil {
		serverError(w, r, "shell", err, "invoices.error.load_failed")
		return
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	httpx.Render(w, r, templates.SupplierFormPage(templates.SupplierPageData{
		Shell:  sh,
		Locale: locale(r),
		CSRF:   httpx.CurrentSession(r).CSRFToken,
		Edit:   edit,
		Values: v,
		Errors: errs,
		Error:  msg,
	}))
}

func (h *Handler) supplierNewGET(w http.ResponseWriter, r *http.Request) {
	h.renderSupplierForm(w, r, http.StatusOK, false, templates.SupplierValues{Country: domain.CountryPT}, nil, "")
}

// supplierSaveError re-renders the supplier form for a validation error.
func (h *Handler) supplierSaveError(w http.ResponseWriter, r *http.Request, edit bool, v templates.SupplierValues, err error) {
	var fe domain.FieldErrors
	switch {
	case errors.As(err, &fe):
		h.renderSupplierForm(w, r, http.StatusUnprocessableEntity, edit, v, fe, "")
	case isNotFound(err):
		fail(w, r, http.StatusNotFound, "suppliers.error.not_found")
	default:
		serverError(w, r, "save supplier", err, "suppliers.error.save_failed")
	}
}

func (h *Handler) supplierCreatePOST(w http.ResponseWriter, r *http.Request) {
	user, condo := actor(r)
	in := supplierFromForm(r, "")
	if _, err := h.deps.Suppliers.Create(r.Context(), user, condo, in); err != nil {
		h.supplierSaveError(w, r, false, supplierValues(in), err)
		return
	}
	http.Redirect(w, r, suppliersPath+"?flash=created", http.StatusSeeOther)
}

func (h *Handler) supplierEditGET(w http.ResponseWriter, r *http.Request) {
	_, condo := actor(r)
	s, err := h.deps.Suppliers.Get(r.Context(), condo, r.PathValue("id"))
	if err != nil {
		h.supplierSaveError(w, r, true, templates.SupplierValues{}, err)
		return
	}
	h.renderSupplierForm(w, r, http.StatusOK, true, supplierValues(s), nil, "")
}

func (h *Handler) supplierEditPOST(w http.ResponseWriter, r *http.Request) {
	user, condo := actor(r)
	id := r.PathValue("id")
	in := supplierFromForm(r, "")
	v := supplierValues(in)
	v.ID = id
	if err := h.deps.Suppliers.Update(r.Context(), user, condo, id, in); err != nil {
		h.supplierSaveError(w, r, true, v, err)
		return
	}
	http.Redirect(w, r, suppliersPath+"?flash=updated", http.StatusSeeOther)
}
