package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/core/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/core/ports"
	"github.com/leoarkiteto/zelo/internal/features/finance/core/services"
	financetemplates "github.com/leoarkiteto/zelo/internal/features/finance/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

// financeGET renders the dashboard and account list with filters.
func (h *Handler) financeGET(w http.ResponseWriter, r *http.Request) {
	data, err := h.financeData(r, flashMessage(r, i18n.LanguageFrom(r.Context())))
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(i18n.LanguageFrom(r.Context()), "finance.error.load"))
		return
	}
	httpx.Render(w, r, financetemplates.FinancePage(data))
}

// financeNewGET renders the account creation form for the requested type.
func (h *Handler) financeNewGET(w http.ResponseWriter, r *http.Request) {
	accType := domain.AccountType(r.URL.Query().Get("type"))
	if !accType.Valid() {
		accType = domain.AccountTypePayable
	}
	data, err := h.formData(r, financetemplates.FormValues{Type: string(accType)}, false, "", accType, "")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(i18n.LanguageFrom(r.Context()), "finance.error.load"))
		return
	}
	httpx.Render(w, r, financetemplates.FinanceFormPage(data))
}

// financeCreatePOST creates an account (multipart when a receipt is attached).
func (h *Handler) financeCreatePOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	in := h.parseAccountForm(r)
	values := formValuesFromInput(in)

	if in.Type == domain.AccountTypePayable {
		path, name, err := h.saveReceipt(r)
		switch {
		case errors.Is(err, errReceiptInvalid):
			h.renderFormWithError(w, r, values, false, "", in.Type, i18n.T(locale, "finance.error.receipt"))
			return
		case err != nil:
			httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.create"))
			return
		default:
			in.ReceiptPath = path
			in.ReceiptName = name
		}
	}

	_, err := h.deps.Finance.CreateAccount(r.Context(), u.ID, sess.CondominiumID, in)
	if err != nil {
		h.renderCreateError(w, r, values, in.Type, err)
		return
	}
	http.Redirect(w, r, "/finance?flash=created", http.StatusSeeOther)
}

// financeEditGET renders the edit form for a pending/overdue account.
func (h *Handler) financeEditGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	account, err := h.accountForCondominium(r, r.PathValue("id"))
	if err != nil {
		h.renderAccountGone(w, r, err)
		return
	}
	if !account.CanEdit() {
		httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "finance.error.not_editable"))
		return
	}
	values := financetemplates.FormValues{
		Type:          string(account.Type),
		Title:         account.Title,
		Category:      string(account.Category),
		Amount:        centsToDecimal(account.AmountCents),
		DueDate:       account.DueDate.Format("2006-01-02"),
		SupplierPayee: account.SupplierPayee,
		PaymentCode:   account.PaymentCode,
		UnitID:        account.UnitID,
	}
	data, err := h.formData(r, values, true, account.ID, account.Type, "")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
		return
	}
	httpx.Render(w, r, financetemplates.FinanceFormPage(data))
}

// financeEditPOST updates an editable account's fields.
func (h *Handler) financeEditPOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	id := r.PathValue("id")

	existing, err := h.accountForCondominium(r, id)
	if err != nil {
		h.renderAccountGone(w, r, err)
		return
	}
	if !existing.CanEdit() {
		httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "finance.error.not_editable"))
		return
	}

	in := h.parseAccountForm(r)
	values := formValuesFromInput(in)

	if in.Type == domain.AccountTypePayable {
		path, name, err := h.saveReceipt(r)
		switch {
		case errors.Is(err, errReceiptInvalid):
			h.renderFormWithError(w, r, values, true, id, in.Type, i18n.T(locale, "finance.error.receipt"))
			return
		case err != nil:
			httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.update"))
			return
		case path != "":
			in.ReceiptPath = path
			in.ReceiptName = name
		default:
			// Keep the existing receipt when no new file is uploaded.
			in.ReceiptPath = existing.ReceiptPath
			in.ReceiptName = existing.ReceiptName
		}
	}

	err = h.deps.Finance.EditAccount(r.Context(), u.ID, id, sess.CondominiumID, in)
	if err != nil {
		h.renderEditError(w, r, id, values, in.Type, err)
		return
	}
	http.Redirect(w, r, "/finance?flash=updated", http.StatusSeeOther)
}

// financeSettleGET renders the settlement form.
func (h *Handler) financeSettleGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	account, err := h.accountForCondominium(r, r.PathValue("id"))
	if err != nil {
		h.renderAccountGone(w, r, err)
		return
	}
	if !account.CanSettle() {
		httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "finance.error.not_editable"))
		return
	}
	httpx.Render(w, r, financetemplates.SettlePage(h.settleData(r, account, "")))
}

// financeSettlePOST settles an account with the transaction date (FR-010).
func (h *Handler) financeSettlePOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	id := r.PathValue("id")
	date := r.FormValue("settlement_date")

	account, err := h.accountForCondominium(r, id)
	if err != nil {
		h.renderAccountGone(w, r, err)
		return
	}
	if err := h.deps.Finance.SettleAccount(r.Context(), u.ID, id, sess.CondominiumID, date); err != nil {
		switch {
		case errors.Is(err, services.ErrSettlementDateRequired):
			httpx.Render(w, r, financetemplates.SettlePage(h.settleData(r, account, i18n.T(locale, "finance.error.settlement_date"))))
		case errors.Is(err, services.ErrNotEditable):
			httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "finance.error.not_editable"))
		case errors.Is(err, services.ErrNotFound):
			httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "finance.error.account_gone"))
		default:
			httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.settle"))
		}
		return
	}
	http.Redirect(w, r, "/finance?flash=settled", http.StatusSeeOther)
}

// financeCancelGET renders the cancel confirmation page.
func (h *Handler) financeCancelGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	account, err := h.accountForCondominium(r, r.PathValue("id"))
	if err != nil {
		h.renderAccountGone(w, r, err)
		return
	}
	if !account.CanCancel() {
		httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "finance.error.not_editable"))
		return
	}
	httpx.Render(w, r, financetemplates.CancelPage(h.cancelData(r, account, "")))
}

// financeCancelPOST cancels a pending/overdue account.
func (h *Handler) financeCancelPOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	id := r.PathValue("id")
	if err := h.deps.Finance.CancelAccount(r.Context(), u.ID, id, sess.CondominiumID); err != nil {
		h.renderAccountGone(w, r, err)
		return
	}
	http.Redirect(w, r, "/finance?flash=canceled", http.StatusSeeOther)
}

// myChargesGET renders the resident's pending charges (FR-016).
func (h *Handler) myChargesGET(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())

	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance/my-charges")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load_session"))
		return
	}
	charges, err := h.deps.Finance.ResidentCharges(r.Context(), u.ID, sess.CondominiumID)
	data := financetemplates.ChargesPageData{
		Shell:  shell,
		Locale: locale,
		CSRF:   sess.CSRFToken,
	}
	switch {
	case errors.Is(err, services.ErrNoUnit):
		data.NoUnit = true
	case err != nil:
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
		return
	default:
		for _, c := range charges {
			data.Charges = append(data.Charges, h.accountView(locale, c))
		}
	}
	httpx.Render(w, r, financetemplates.ChargesPage(data))
}

// healthGET renders the aggregated financial health (FR-017).
func (h *Handler) healthGET(w http.ResponseWriter, r *http.Request) {
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())

	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance/health")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load_session"))
		return
	}
	month := currentMonth(time.Now())
	period := r.URL.Query().Get("period")
	if m, err := time.Parse("2006-01", period); err == nil {
		month = time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	sum, err := h.deps.Finance.Health(r.Context(), sess.CondominiumID, month)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
		return
	}
	data := financetemplates.HealthPageData{
		Shell:         shell,
		Locale:        locale,
		CSRF:          sess.CSRFToken,
		Period:        month.Format("2006-01"),
		TotalSpent:    financetemplates.Money(locale, sum.RealizedPayableCents),
		TotalCollected: financetemplates.Money(locale, sum.RealizedReceivableCents),
		Empty:         sum.RealizedPayableCents == 0 && sum.RealizedReceivableCents == 0,
	}
	httpx.Render(w, r, financetemplates.HealthPage(data))
}

// --- shared helpers ---

func (h *Handler) financeData(r *http.Request, flash string) (financetemplates.FinancePageData, error) {
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance")
	if err != nil {
		return financetemplates.FinancePageData{}, err
	}

	q := r.URL.Query()
	f := ports.AccountFilter{Query: q.Get("q")}
	if t := domain.AccountType(q.Get("type")); t.Valid() {
		f.Type = t
	}
	if c := domain.Category(q.Get("category")); c != "" {
		f.Category = c
	}
	rawStatus := q.Get("status")
	switch rawStatus {
	case "inadimplentes":
		// Dedicated delinquents filter: only overdue receivables (FR-015/SC-005).
		f.Type = domain.AccountTypeReceivable
		f.Status = domain.StatusOverdue
	case "":
	default:
		f.Status = domain.AccountStatus(rawStatus)
	}
	month := currentMonth(time.Now())
	if p := q.Get("period"); p != "" {
		if m, err := time.Parse("2006-01", p); err == nil {
			month = time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, time.UTC)
			f.Month = month
		}
	}

	accounts, err := h.deps.Finance.ListAccounts(r.Context(), sess.CondominiumID, f)
	if err != nil {
		return financetemplates.FinancePageData{}, err
	}
	sum, err := h.deps.Finance.Summary(r.Context(), sess.CondominiumID, month)
	if err != nil {
		return financetemplates.FinancePageData{}, err
	}

	// Category filter shows the categories of the selected type, or the union
	// of both when no type filter is set (FR-014).
	var filterCats []domain.Category
	if f.Type != "" {
		filterCats = domain.CategoriesFor(f.Type)
	}

	data := financetemplates.FinancePageData{
		Shell:      shell,
		Locale:     locale,
		CSRF:       sess.CSRFToken,
		Query:      f.Query,
		Type:       string(f.Type),
		Category:   string(f.Category),
		Status:     rawStatus,
		Period:     month.Format("2006-01"),
		Categories: financetemplates.CategoryOptions(locale, filterCats),
		Statuses:   financetemplates.StatusOptions(locale),
		Types:      financetemplates.TypeOptions(locale),
		Summary: financetemplates.SummaryView{
			TotalReceivable: financetemplates.Money(locale, sum.TotalReceivableCents),
			TotalPayable:    financetemplates.Money(locale, sum.TotalPayableCents),
			ProjectedBalance: financetemplates.Money(locale, sum.ProjectedBalanceCents),
			RealizedBalance:  financetemplates.Money(locale, sum.RealizedBalanceCents),
		},
		Flash: flash,
	}
	for _, a := range accounts {
		data.Accounts = append(data.Accounts, h.accountView(locale, a))
	}
	return data, nil
}

func (h *Handler) accountView(locale i18n.Language, a domain.FinancialAccount) financetemplates.AccountView {
	view := financetemplates.AccountView{
		ID:            a.ID,
		Type:          a.Type,
		Title:         a.Title,
		CategoryLabel: financetemplates.CategoryLabel(locale, a.Type, a.Category),
		Amount:        financetemplates.Money(locale, a.AmountCents),
		DueDate:       i18n.FormatDate(locale, a.DueDate),
		EffectiveStatus: a.EffectiveStatus(time.Now()),
		StatusLabel:   financetemplates.StatusLabel(locale, a.EffectiveStatus(time.Now())),
		SupplierPayee: a.SupplierPayee,
		HasReceipt:    a.ReceiptPath != "",
	}
	if a.SettlementDate != nil {
		view.SettlementDate = i18n.FormatDate(locale, *a.SettlementDate)
	}
	return view
}

func (h *Handler) parseAccountForm(r *http.Request) services.AccountInput {
	return services.AccountInput{
		Type:           domain.AccountType(r.FormValue("type")),
		Title:          r.FormValue("title"),
		Category:       domain.Category(r.FormValue("category")),
		Amount:         r.FormValue("amount"),
		DueDate:        r.FormValue("due_date"),
		Status:         domain.AccountStatus(r.FormValue("status")),
		SettlementDate: r.FormValue("settlement_date"),
		SupplierPayee:  r.FormValue("supplier_payee"),
		PaymentCode:    r.FormValue("payment_code"),
		UnitID:         r.FormValue("unit_id"),
	}
}

func formValuesFromInput(in services.AccountInput) financetemplates.FormValues {
	return financetemplates.FormValues{
		Type:           string(in.Type),
		Title:          in.Title,
		Category:       string(in.Category),
		Amount:         in.Amount,
		DueDate:        in.DueDate,
		Status:         string(in.Status),
		SettlementDate: in.SettlementDate,
		SupplierPayee:  in.SupplierPayee,
		PaymentCode:    in.PaymentCode,
		UnitID:         in.UnitID,
	}
}

func (h *Handler) formData(r *http.Request, values financetemplates.FormValues, isEdit bool, accountID string, accType domain.AccountType, errorMsg string) (financetemplates.FinanceFormData, error) {
	sess := httpx.CurrentSession(r)
	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance")
	if err != nil {
		return financetemplates.FinanceFormData{}, err
	}
	units, err := h.deps.Units.ListUnitsForCondominium(r.Context(), sess.CondominiumID)
	if err != nil {
		return financetemplates.FinanceFormData{}, err
	}
	data := financetemplates.FinanceFormData{
		Shell:       shell,
		Locale:      i18n.LanguageFrom(r.Context()),
		CSRF:        sess.CSRFToken,
		IsEdit:      isEdit,
		AccountID:   accountID,
		AccountType: accType,
		Values:      values,
		Categories:  financetemplates.CategoryOptions(dataLocale(r), domain.CategoriesFor(accType)),
		Units:       financetemplates.UnitOptions(units),
		Error:       errorMsg,
	}
	return data, nil
}

func (h *Handler) renderFormWithError(w http.ResponseWriter, r *http.Request, values financetemplates.FormValues, isEdit bool, accountID string, accType domain.AccountType, errorMsg string) {
	data, err := h.formData(r, values, isEdit, accountID, accType, errorMsg)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(i18n.LanguageFrom(r.Context()), "finance.error.load"))
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	httpx.Render(w, r, financetemplates.FinanceFormPage(data))
}

func (h *Handler) renderCreateError(w http.ResponseWriter, r *http.Request, values financetemplates.FormValues, accType domain.AccountType, err error) {
	locale := i18n.LanguageFrom(r.Context())
	switch {
	case errors.Is(err, services.ErrInvalidInput):
		h.renderFormWithError(w, r, values, false, "", accType, i18n.T(locale, "finance.error.required"))
	case errors.Is(err, services.ErrInvalidAmount):
		h.renderFormWithError(w, r, values, false, "", accType, i18n.T(locale, "finance.error.amount"))
	case errors.Is(err, services.ErrInvalidCategory):
		h.renderFormWithError(w, r, values, false, "", accType, i18n.T(locale, "finance.error.category"))
	case errors.Is(err, services.ErrInvalidDueDate):
		h.renderFormWithError(w, r, values, false, "", accType, i18n.T(locale, "finance.error.due_date"))
	case errors.Is(err, services.ErrSettlementDateRequired):
		h.renderFormWithError(w, r, values, false, "", accType, i18n.T(locale, "finance.error.settlement_date"))
	case errors.Is(err, services.ErrInvalidUnit):
		h.renderFormWithError(w, r, values, false, "", accType, i18n.T(locale, "finance.error.unit"))
	default:
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.create"))
	}
}

func (h *Handler) renderEditError(w http.ResponseWriter, r *http.Request, id string, values financetemplates.FormValues, accType domain.AccountType, err error) {
	locale := i18n.LanguageFrom(r.Context())
	switch {
	case errors.Is(err, services.ErrInvalidInput):
		h.renderFormWithError(w, r, values, true, id, accType, i18n.T(locale, "finance.error.required"))
	case errors.Is(err, services.ErrInvalidAmount):
		h.renderFormWithError(w, r, values, true, id, accType, i18n.T(locale, "finance.error.amount"))
	case errors.Is(err, services.ErrInvalidCategory):
		h.renderFormWithError(w, r, values, true, id, accType, i18n.T(locale, "finance.error.category"))
	case errors.Is(err, services.ErrInvalidDueDate):
		h.renderFormWithError(w, r, values, true, id, accType, i18n.T(locale, "finance.error.due_date"))
	case errors.Is(err, services.ErrInvalidUnit):
		h.renderFormWithError(w, r, values, true, id, accType, i18n.T(locale, "finance.error.unit"))
	case errors.Is(err, services.ErrNotEditable):
		httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "finance.error.not_editable"))
	case errors.Is(err, services.ErrNotFound):
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "finance.error.account_gone"))
	default:
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.update"))
	}
}

func (h *Handler) settleData(r *http.Request, account domain.FinancialAccount, errorMsg string) financetemplates.SettleData {
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	shell, _ := httpx.ShellData(r, h.deps.Roles, "/finance")
	return financetemplates.SettleData{
		Shell:   shell,
		Locale:  locale,
		CSRF:    sess.CSRFToken,
		Account: h.accountView(locale, account),
		Error:   errorMsg,
	}
}

func (h *Handler) cancelData(r *http.Request, account domain.FinancialAccount, errorMsg string) financetemplates.CancelData {
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	shell, _ := httpx.ShellData(r, h.deps.Roles, "/finance")
	return financetemplates.CancelData{
		Shell:   shell,
		Locale:  locale,
		CSRF:    sess.CSRFToken,
		Account: h.accountView(locale, account),
		Error:   errorMsg,
	}
}

// accountForCondominium loads an account and verifies it belongs to the
// session's condominium.
func (h *Handler) accountForCondominium(r *http.Request, id string) (domain.FinancialAccount, error) {
	account, err := h.deps.Accounts.GetByID(r.Context(), id)
	if err != nil {
		return domain.FinancialAccount{}, err
	}
	if account.CondominiumID != httpx.CurrentSession(r).CondominiumID {
		return domain.FinancialAccount{}, services.ErrNotFound
	}
	return account, nil
}

func (h *Handler) renderAccountGone(w http.ResponseWriter, r *http.Request, err error) {
	locale := i18n.LanguageFrom(r.Context())
	switch {
	case errors.Is(err, services.ErrNotFound):
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "finance.error.account_gone"))
	case errors.Is(err, services.ErrNotEditable):
		httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "finance.error.not_editable"))
	default:
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
	}
}

func currentMonth(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

// centsToDecimal renders cents as a decimal string for form prefill.
func centsToDecimal(cents int64) string {
	whole := cents / 100
	frac := cents % 100
	return fmt.Sprintf("%d.%02d", whole, frac)
}

// flashMessage maps a ?flash= query value to a user-facing confirmation.
func flashMessage(r *http.Request, locale i18n.Language) string {
	switch r.URL.Query().Get("flash") {
	case "created":
		return i18n.T(locale, "finance.flash.created")
	case "updated":
		return i18n.T(locale, "finance.flash.updated")
	case "settled":
		return i18n.T(locale, "finance.flash.settled")
	case "canceled":
		return i18n.T(locale, "finance.flash.canceled")
	default:
		return ""
	}
}

func dataLocale(r *http.Request) i18n.Language { return i18n.LanguageFrom(r.Context()) }
