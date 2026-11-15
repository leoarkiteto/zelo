package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
)

// The fragment handlers return only the HTML an htmx swap needs, never a full
// page, and report problems inside the fragment.

// sectionGET returns the conditional section of an expense type (FR-10).
func (h *Handler) sectionGET(w http.ResponseWriter, r *http.Request) {
	pf := parseInvoiceForm(r)
	httpx.Render(w, r, templates.ExpenseSection(templates.ExpenseSectionData{
		Locale:      pf.Locale,
		ExpenseType: pf.ExpenseType,
		Values:      pf.Values,
	}))
}

// linesPOST re-renders the line table from the posted form. op=add appends a
// blank line, op=remove drops line index, and an empty op just re-renders (the
// columns and rate options depend on the expense type, region and treatment).
func (h *Handler) linesPOST(w http.ResponseWriter, r *http.Request) {
	pf := parseInvoiceForm(r)
	applyLineOp(pf, strings.TrimSpace(r.FormValue("op")), r.FormValue("index"))
	resetDisallowedRates(pf)
	httpx.Render(w, r, templates.LinesFragment(templates.LinesFragmentData{
		Locale:      pf.Locale,
		ExpenseType: pf.ExpenseType,
		Region:      pf.Region,
		Lines:       pf.Lines,
	}))
}

// previewPOST computes the summary of the posted lines on the server (FR-30).
// An empty or invalid form yields the message inside the summary, not an HTTP
// error.
func (h *Handler) previewPOST(w http.ResponseWriter, r *http.Request) {
	pf := parseInvoiceForm(r)
	httpx.Render(w, r, templates.SummaryFragment(h.summaryFromForm(r, pf)))
}

// supplierSelectedGET returns the selected-supplier fragment of one supplier of
// the session's condominium (FR-7, I11). A supplier of another condominium is
// a 404.
func (h *Handler) supplierSelectedGET(w http.ResponseWriter, r *http.Request) {
	_, condo := actor(r)
	sup, err := h.deps.Suppliers.Get(r.Context(), condo, r.PathValue("id"))
	switch {
	case err == nil:
		httpx.Render(w, r, templates.SupplierSelected(templates.SupplierSelectedData{
			Locale: locale(r), Supplier: supplierValues(sup),
		}))
	case isNotFound(err) && middleware.IsHX(r):
		// htmx v4 swaps 4xx bodies, so an HX request gets the fragment itself
		// with the message instead of a full error page (I2).
		w.WriteHeader(http.StatusNotFound)
		httpx.Render(w, r, templates.SupplierSelected(templates.SupplierSelectedData{
			Locale: locale(r),
			Errors: domain.FieldErrors{"supplier_id": "suppliers.error.not_found"},
		}))
	case isNotFound(err):
		notFound(w, r)
	default:
		serverError(w, r, "select supplier", err, "invoices.error.load_failed")
	}
}

// supplierSearchGET serves the supplier autocomplete (q) for the invoice form
// (FR-7). A failed search is logged and renders empty results.
func (h *Handler) supplierSearchGET(w http.ResponseWriter, r *http.Request) {
	_, condo := actor(r)
	loc := locale(r)

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		return
	}
	results, err := h.deps.Suppliers.Search(r.Context(), condo, q, 10)
	if err != nil {
		slog.Error("invoices: supplier search", "error", err)
		results = nil
	}
	httpx.Render(w, r, templates.SupplierSearchResults(templates.SupplierSearchData{
		Locale: loc, Query: q, Results: results,
	}))
}

// supplierInlinePOST creates a supplier from the invoice form and returns the
// selected-supplier fragment (FR-7). A rejected supplier answers 422 with the
// first error shown in the fragment.
func (h *Handler) supplierInlinePOST(w http.ResponseWriter, r *http.Request) {
	user, condo := actor(r)
	loc := locale(r)
	in := supplierFromForm(r, "supplier_")

	sup, err := h.deps.Suppliers.Create(r.Context(), user, condo, in)
	if err == nil {
		httpx.Render(w, r, templates.SupplierSelected(templates.SupplierSelectedData{
			Locale: loc, Supplier: supplierValues(sup),
		}))
		return
	}
	key := "suppliers.error.save_failed"
	var fe domain.FieldErrors
	if errors.As(err, &fe) {
		for _, f := range []string{"name", "nif"} {
			if k, ok := fe[f]; ok {
				key = k
				break
			}
		}
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	httpx.Render(w, r, templates.SupplierSelected(templates.SupplierSelectedData{
		Locale: loc,
		Errors: domain.FieldErrors{"supplier_id": key},
	}))
}
