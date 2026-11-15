package handlers

import (
	"bytes"
	"context"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/services"
	"github.com/leoarkiteto/zelo/internal/features/invoices/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

const listPath = "/invoices"

func invoicePath(id string) string { return listPath + "/" + url.PathEscape(id) }

// filterFromQuery reads the list filters (FR-28) and the page number.
func filterFromQuery(r *http.Request) domain.ListFilter {
	q := r.URL.Query()
	f := domain.ListFilter{
		SupplierName:    strings.TrimSpace(q.Get("supplier")),
		NIF:             strings.TrimSpace(q.Get("nif")),
		Number:          strings.TrimSpace(q.Get("number")),
		DocumentType:    q.Get("document_type"),
		ExpenseType:     q.Get("expense_type"),
		Category:        q.Get("category"),
		Status:          q.Get("status"),
		IssueFrom:       q.Get("issue_from"),
		IssueTo:         q.Get("issue_to"),
		DueFrom:         q.Get("due_from"),
		DueTo:           q.Get("due_to"),
		ReferencePeriod: q.Get("period"),
	}
	if p, err := strconv.Atoi(q.Get("page")); err == nil {
		f.Page = p
	}
	return f.Normalized()
}

// listGET renders the invoice list: filters, pagination, totals.
func (h *Handler) listGET(w http.ResponseWriter, r *http.Request) {
	_, condo := actor(r)
	f := filterFromQuery(r)
	res, err := h.deps.Invoices.List(r.Context(), condo, f)
	if err != nil {
		serverError(w, r, "list", err, "invoices.error.load_failed")
		return
	}
	sh, err := shell(r, listPath)
	if err != nil {
		serverError(w, r, "shell", err, "invoices.error.load_failed")
		return
	}
	httpx.Render(w, r, templates.ListPage(templates.ListPageData{
		ListResult: res,
		Shell:      sh,
		Locale:     locale(r),
		CSRF:       httpx.CurrentSession(r).CSRFToken,
		Filter:     f,
		Flash:      flashText(r, "invoices.flash.", "draft_deleted"),
	}))
}

// formState is everything the invoice form page renders.
type formState struct {
	Edit    bool
	ID      string
	State   domain.State
	Values  templates.FormValues
	Lines   []templates.LineForm
	Errors  domain.FieldErrors
	Message string // page-level error, already translated
	// DuplicateID, when set, adds a link to the existing invoice (FR-23).
	DuplicateID string
	Summary     templates.SummaryData

	Payments    []domain.Payment
	Attachments []domain.Attachment
}

// renderForm renders the full create or edit page with the given status.
func (h *Handler) renderForm(w http.ResponseWriter, r *http.Request, status int, st formState) {
	_, condo := actor(r)
	sh, err := shell(r, listPath)
	if err != nil {
		serverError(w, r, "shell", err, "invoices.error.load_failed")
		return
	}
	var sup templates.SupplierValues
	if st.Values.SupplierID != "" {
		if s, err := h.deps.Suppliers.Get(r.Context(), condo, st.Values.SupplierID); err == nil {
			sup = supplierValues(s)
		}
	}
	if st.Summary.Locale == "" {
		st.Summary.Locale = locale(r)
	}
	comp := templates.FormPage(templates.FormData{
		Shell:       sh,
		Locale:      locale(r),
		CSRF:        httpx.CurrentSession(r).CSRFToken,
		Edit:        st.Edit,
		InvoiceID:   st.ID,
		State:       st.State,
		Values:      st.Values,
		Supplier:    sup,
		Lines:       st.Lines,
		Summary:     st.Summary,
		Errors:      st.Errors,
		Error:       st.Message,
		Payments:    st.Payments,
		Attachments: st.Attachments,
	})
	if st.DuplicateID != "" {
		comp = withDuplicateLink(comp, r, st.DuplicateID)
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	httpx.Render(w, r, comp)
}

// withDuplicateLink inserts a link to the existing invoice right before the
// form, under the duplicate message. FormData carries only a text message, so
// the link is added to the rendered page.
func withDuplicateLink(page templ.Component, r *http.Request, existingID string) templ.Component {
	link := `<p class="alert alert-info" data-duplicate-link><a class="text-steel-700 underline" href="` +
		html.EscapeString(invoicePath(existingID)) + `">` +
		html.EscapeString(tr(r, "invoices.flash.duplicate_link")) + `</a></p>`
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		var buf bytes.Buffer
		if err := page.Render(ctx, &buf); err != nil {
			return err
		}
		out := buf.String()
		const anchor = `<form id="invoice-form"`
		if i := strings.Index(out, anchor); i >= 0 {
			out = out[:i] + link + out[i:]
		} else {
			out += link
		}
		_, err := w.Write([]byte(out))
		return err
	})
}

// summaryFromForm computes the financial summary of the posted lines. Parse
// errors of lines or withholding, and rule errors, become SummaryData.Error:
// the preview never fails the request.
func (h *Handler) summaryFromForm(r *http.Request, pf *parsedForm) templates.SummaryData {
	sd := templates.SummaryData{Locale: pf.Locale}
	lineErrs := domain.FieldErrors{}
	for k, v := range pf.Errors {
		if strings.HasPrefix(k, "lines[") || k == "withholding" {
			lineErrs[k] = v
		}
	}
	if len(lineErrs) > 0 {
		sd.Error = firstMessage(pf.Locale, remapFieldErrors(lineErrs, pf.ExpenseType == domain.ExpenseCompraProdutos))
		return sd
	}
	totals, err := h.deps.Invoices.Preview(pf.Region, pf.Invoice.Lines, pf.Withholding)
	if err != nil {
		var fe domain.FieldErrors
		if errors.As(err, &fe) {
			sd.Error = firstMessage(pf.Locale, remapFieldErrors(fe, pf.ExpenseType == domain.ExpenseCompraProdutos))
		} else {
			sd.Error = tr(r, "invoices.error.save_failed")
		}
		return sd
	}
	sd.Totals = totals
	return sd
}

// newGET renders the empty create form with a fresh idempotency key.
func (h *Handler) newGET(w http.ResponseWriter, r *http.Request) {
	h.renderForm(w, r, http.StatusOK, formState{
		Values: templates.FormValues{
			IdempotencyKey: newIdempotencyKey(),
			DocumentType:   string(domain.DocumentFatura),
			ExpenseType:    string(domain.ExpenseCompraProdutos),
			FiscalRegion:   string(domain.RegionContinental),
		},
		Lines:   []templates.LineForm{blankLine()},
		Summary: templates.SummaryData{Locale: locale(r)},
	})
}

// formErrorState re-renders the posted form: input preserved, inline errors.
func (h *Handler) formErrorState(r *http.Request, pf *parsedForm, errs domain.FieldErrors) formState {
	return formState{
		Values:  pf.Values,
		Lines:   pf.Lines,
		Errors:  errs,
		Summary: h.summaryFromForm(r, pf),
	}
}

// saveError maps a failed create or edit to a response: validation errors and
// rule violations re-render the form with 422, a duplicate adds the link to the
// existing invoice, and anything else is a server error.
func (h *Handler) saveError(w http.ResponseWriter, r *http.Request, pf *parsedForm, st formState, err error) {
	var dup *services.DuplicateInvoiceError
	var fe domain.FieldErrors
	switch {
	case errors.As(err, &dup):
		st.Message = tr(r, "invoices.flash.duplicate")
		st.DuplicateID = dup.ExistingID
		h.renderForm(w, r, http.StatusUnprocessableEntity, st)
	case errors.As(err, &fe):
		st.Errors = remapFieldErrors(fe, pf.ExpenseType == domain.ExpenseCompraProdutos)
		h.renderForm(w, r, http.StatusUnprocessableEntity, st)
	case isNotFound(err):
		notFound(w, r)
	case ruleKey(err) != "":
		st.Message = tr(r, ruleKey(err))
		h.renderForm(w, r, http.StatusUnprocessableEntity, st)
	default:
		serverError(w, r, "save", err, "invoices.error.save_failed")
	}
}

// createPOST creates a draft or a registered invoice by the action field. The
// idempotency key makes a double submit create only one invoice (FR-24).
func (h *Handler) createPOST(w http.ResponseWriter, r *http.Request) {
	user, condo := actor(r)
	pf := parseInvoiceForm(r)
	if pf.Invoice.IdempotencyKey == "" {
		pf.Invoice.IdempotencyKey = newIdempotencyKey()
		pf.Values.IdempotencyKey = pf.Invoice.IdempotencyKey
	}
	register := r.FormValue("action") == "register"
	st := h.formErrorState(r, pf, nil)

	if len(pf.Errors) > 0 {
		st.Errors = remapFieldErrors(pf.Errors, pf.ExpenseType == domain.ExpenseCompraProdutos)
		h.renderForm(w, r, http.StatusUnprocessableEntity, st)
		return
	}

	var res services.SaveResult
	var err error
	if register {
		res, err = h.deps.Invoices.Register(r.Context(), user, condo, pf.Invoice)
	} else {
		res, err = h.deps.Invoices.SaveDraft(r.Context(), user, condo, pf.Invoice)
	}
	if err != nil {
		h.saveError(w, r, pf, st, err)
		return
	}
	if res.Existing {
		http.Redirect(w, r, invoicePath(res.ID), http.StatusSeeOther)
		return
	}
	flash := "saved"
	if register {
		flash = "registered"
	}
	http.Redirect(w, r, invoicePath(res.ID)+"?flash="+flash, http.StatusSeeOther)
}

// editGET renders the edit form of a draft or registered invoice.
func (h *Handler) editGET(w http.ResponseWriter, r *http.Request) {
	_, condo := actor(r)
	id := r.PathValue("id")
	inv, err := h.deps.Invoices.Get(r.Context(), condo, id)
	if err != nil {
		h.loadError(w, r, err)
		return
	}
	if inv.State == domain.StateAnnulled {
		http.Redirect(w, r, invoicePath(inv.ID), http.StatusSeeOther)
		return
	}
	loc := locale(r)
	lines := lineFormsFromInvoice(loc, inv)
	if len(lines) == 0 {
		lines = []templates.LineForm{blankLine()}
	}
	h.renderForm(w, r, http.StatusOK, formState{
		Edit: true, ID: inv.ID, State: inv.State,
		Values:      valuesFromInvoice(loc, inv),
		Lines:       lines,
		Summary:     templates.SummaryData{Locale: loc, Totals: inv.Totals},
		Payments:    inv.Payments,
		Attachments: inv.Attachments,
	})
}

func (h *Handler) loadError(w http.ResponseWriter, r *http.Request, err error) {
	if isNotFound(err) {
		notFound(w, r)
		return
	}
	serverError(w, r, "load", err, "invoices.error.load_failed")
}

// editPOST saves an edit. A draft is saved as a draft unless action=register.
func (h *Handler) editPOST(w http.ResponseWriter, r *http.Request) {
	user, condo := actor(r)
	id := r.PathValue("id")
	existing, err := h.deps.Invoices.Get(r.Context(), condo, id)
	if err != nil {
		h.loadError(w, r, err)
		return
	}
	if existing.State == domain.StateAnnulled {
		http.Redirect(w, r, invoicePath(existing.ID), http.StatusSeeOther)
		return
	}
	pf := parseInvoiceForm(r)
	register := r.FormValue("action") == "register"
	st := h.formErrorState(r, pf, nil)
	st.Edit, st.ID, st.State = true, existing.ID, existing.State
	st.Payments, st.Attachments = existing.Payments, existing.Attachments

	if len(pf.Errors) > 0 {
		st.Errors = remapFieldErrors(pf.Errors, pf.ExpenseType == domain.ExpenseCompraProdutos)
		h.renderForm(w, r, http.StatusUnprocessableEntity, st)
		return
	}
	if err := h.deps.Invoices.Edit(r.Context(), user, condo, id, pf.Invoice, register); err != nil {
		h.saveError(w, r, pf, st, err)
		return
	}
	flash := "saved"
	if register && existing.State == domain.StateDraft {
		flash = "registered"
	}
	http.Redirect(w, r, invoicePath(id)+"?flash="+flash, http.StatusSeeOther)
}

// viewState is everything the view page renders besides the invoice.
type viewState struct {
	Payment       templates.PaymentValues
	PaymentErrors domain.FieldErrors
	AnnulReason   string
	AnnulError    string
	AttachError   string
	Message       string
	Flash         string
}

func (h *Handler) renderView(w http.ResponseWriter, r *http.Request, status int, inv domain.Invoice, vs viewState) {
	sh, err := shell(r, listPath)
	if err != nil {
		serverError(w, r, "shell", err, "invoices.error.load_failed")
		return
	}
	if vs.Payment.Date == "" && vs.PaymentErrors == nil {
		vs.Payment.Date = time.Now().Format(dateLayout)
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	httpx.Render(w, r, templates.ViewPage(templates.ViewPageData{
		Shell:         sh,
		Locale:        locale(r),
		CSRF:          httpx.CurrentSession(r).CSRFToken,
		Invoice:       inv,
		Flash:         vs.Flash,
		Error:         vs.Message,
		Payment:       vs.Payment,
		PaymentErrors: vs.PaymentErrors,
		AnnulReason:   vs.AnnulReason,
		AnnulError:    vs.AnnulError,
		AttachError:   vs.AttachError,
	}))
}

// viewGET renders one invoice with its lines, summary, payments and files.
func (h *Handler) viewGET(w http.ResponseWriter, r *http.Request) {
	_, condo := actor(r)
	inv, err := h.deps.Invoices.Get(r.Context(), condo, r.PathValue("id"))
	if err != nil {
		h.loadError(w, r, err)
		return
	}
	h.renderView(w, r, http.StatusOK, inv, viewState{
		Flash: flashText(r, "invoices.flash.", "saved", "registered", "annulled", "payment_added", "attachment_added", "attachment_removed"),
	})
}

// deletePOST deletes a draft.
func (h *Handler) deletePOST(w http.ResponseWriter, r *http.Request) {
	user, condo := actor(r)
	id := r.PathValue("id")
	err := h.deps.Invoices.DeleteDraft(r.Context(), user, condo, id)
	switch {
	case err == nil:
		http.Redirect(w, r, listPath+"?flash=draft_deleted", http.StatusSeeOther)
	case isNotFound(err):
		notFound(w, r)
	case errors.Is(err, services.ErrNotDraft):
		fail(w, r, http.StatusConflict, "invoices.error.save_failed")
	default:
		serverError(w, r, "delete draft", err, "invoices.error.save_failed")
	}
}

// annulPOST annuls a registered invoice with a reason (FR-22).
func (h *Handler) annulPOST(w http.ResponseWriter, r *http.Request) {
	user, condo := actor(r)
	id := r.PathValue("id")
	reason := strings.TrimSpace(r.FormValue("annul_reason"))

	inv, err := h.deps.Invoices.Get(r.Context(), condo, id)
	if err != nil {
		h.loadError(w, r, err)
		return
	}
	if reason == "" {
		h.renderView(w, r, http.StatusUnprocessableEntity, inv, viewState{AnnulError: domain.KeyAnnulReasonRequired})
		return
	}
	err = h.deps.Invoices.Annul(r.Context(), user, condo, id, reason)
	switch {
	case err == nil:
		http.Redirect(w, r, invoicePath(id)+"?flash=annulled", http.StatusSeeOther)
	case isNotFound(err):
		notFound(w, r)
	case errors.Is(err, domain.ErrAnnulReasonRequired):
		h.renderView(w, r, http.StatusUnprocessableEntity, inv, viewState{AnnulReason: reason, AnnulError: domain.KeyAnnulReasonRequired})
	case ruleKey(err) != "":
		h.renderView(w, r, http.StatusUnprocessableEntity, inv, viewState{AnnulReason: reason, Message: tr(r, ruleKey(err))})
	default:
		serverError(w, r, "annul", err, "invoices.error.save_failed")
	}
}

// paymentPOST adds a payment. An over-payment needs a justification (FR-17).
func (h *Handler) paymentPOST(w http.ResponseWriter, r *http.Request) {
	user, condo := actor(r)
	id := r.PathValue("id")
	loc := locale(r)

	inv, err := h.deps.Invoices.Get(r.Context(), condo, id)
	if err != nil {
		h.loadError(w, r, err)
		return
	}
	pv := templates.PaymentValues{
		Date:          r.FormValue("payment_date"),
		Amount:        r.FormValue("payment_amount"),
		Method:        r.FormValue("payment_method"),
		Reference:     r.FormValue("payment_reference"),
		BankAccount:   r.FormValue("payment_bank_account"),
		Justification: r.FormValue("payment_justification"),
	}
	errs := domain.FieldErrors{}
	p := domain.Payment{
		Method:        domain.PaymentMethod(strings.TrimSpace(pv.Method)),
		Reference:     strings.TrimSpace(pv.Reference),
		BankAccount:   strings.TrimSpace(pv.BankAccount),
		Justification: strings.TrimSpace(pv.Justification),
	}
	if d := strings.TrimSpace(pv.Date); d != "" {
		t, err := time.Parse(dateLayout, d)
		if err != nil {
			errs.Add("payment_date", domain.KeyInvalid)
		}
		p.Date = t
	}
	if c, err := money.ParseCents(loc, pv.Amount); err != nil {
		errs.Add("payment_amount", domain.KeyInvalidAmount)
	} else {
		p.AmountCents = c
	}
	if !p.Method.Valid() {
		errs.Add("payment_method", domain.KeyRequired)
	}
	if len(errs) > 0 {
		h.renderView(w, r, http.StatusUnprocessableEntity, inv, viewState{Payment: pv, PaymentErrors: errs})
		return
	}

	_, err = h.deps.Invoices.AddPayment(r.Context(), user, condo, id, p)
	if err == nil {
		http.Redirect(w, r, invoicePath(id)+"?flash=payment_added", http.StatusSeeOther)
		return
	}
	vs := viewState{Payment: pv, PaymentErrors: domain.FieldErrors{}}
	var fe domain.FieldErrors
	switch {
	case isNotFound(err):
		notFound(w, r)
		return
	case errors.Is(err, domain.ErrOverpayment):
		vs.PaymentErrors["payment_justification"] = domain.KeyOverpayment
	case errors.Is(err, domain.ErrInvalidAmount):
		vs.PaymentErrors["payment_amount"] = domain.KeyInvalidAmount
	case errors.As(err, &fe):
		for k, v := range fe {
			if k == "method" {
				k = "payment_method"
			}
			vs.PaymentErrors[k] = v
		}
	case ruleKey(err) != "":
		vs.Message = tr(r, ruleKey(err))
	default:
		serverError(w, r, "add payment", err, "invoices.error.save_failed")
		return
	}
	h.renderView(w, r, http.StatusUnprocessableEntity, inv, vs)
}
