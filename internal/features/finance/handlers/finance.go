package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	financetemplates "github.com/leoarkiteto/zelo/internal/features/finance/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/pagination"
	molecules "github.com/leoarkiteto/zelo/internal/shared/templates/molecules"
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
	if h.refuseQuota(w, r, account.ID, true) {
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
	if h.refuseQuota(w, r, existing.ID, true) {
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
	// Quota charges settle through the quota use case, which decides the late
	// fee (R6); every other receivable and payable keeps the generic path.
	settle := func() error { return h.deps.Finance.SettleAccount(r.Context(), u.ID, id, sess.CondominiumID, date) }
	if h.deps.Quotas != nil {
		quota, qerr := h.deps.Finance.IsQuotaAccount(r.Context(), id)
		if qerr != nil {
			httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
			return
		}
		if quota {
			settle = func() error {
				on, perr := domain.ParseSettlementDate(date)
				if perr != nil {
					return perr
				}
				return h.deps.Quotas.SettleQuota(r.Context(), u.ID, sess.CondominiumID, id, on, r.FormValue("late_fee_decision"))
			}
		}
	}
	if err := settle(); err != nil {
		switch {
		case errors.Is(err, services.ErrLateFeeDecisionRequired):
			w.WriteHeader(http.StatusUnprocessableEntity)
			httpx.Render(w, r, financetemplates.SettlePage(h.settleData(r, account, i18n.T(locale, "quotas.error.late_fee_decision_required"))))
		case errors.Is(err, services.ErrSettlementDateRequired), errors.Is(err, domain.ErrInvalidSettlementDate):
			w.WriteHeader(http.StatusUnprocessableEntity)
			httpx.Render(w, r, financetemplates.SettlePage(h.settleData(r, account, i18n.T(locale, "finance.error.settlement_date"))))
		case errors.Is(err, services.ErrInvalidInput):
			w.WriteHeader(http.StatusBadRequest)
			httpx.Render(w, r, financetemplates.SettlePage(h.settleData(r, account, i18n.T(locale, "quotas.error.invalid_decision"))))
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
	// A late fee can be canceled here (a later waiver); only quotas are refused.
	if h.refuseQuota(w, r, account.ID, false) {
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

// --- shared helpers ---

func (h *Handler) financeData(r *http.Request, flash string) (financetemplates.FinancePageData, error) {
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance")
	if err != nil {
		return financetemplates.FinancePageData{}, err
	}

	q := r.URL.Query()
	f := services.AccountFilter{Query: q.Get("q")}
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
	month := currentMonth(h.now())
	if p := q.Get("period"); p != "" {
		if m, err := time.Parse("2006-01", p); err == nil {
			month = time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, time.UTC)
			f.Month = month
		}
	}

	accounts, page, err := h.deps.Finance.ListAccountsPageWithFlags(r.Context(), sess.CondominiumID, f, pagination.Parse(q.Get("page")))
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
			TotalReceivable:  financetemplates.Money(r.Context(), locale, sum.TotalReceivableCents),
			TotalPayable:     financetemplates.Money(r.Context(), locale, sum.TotalPayableCents),
			ProjectedBalance: financetemplates.Money(r.Context(), locale, sum.ProjectedBalanceCents),
			RealizedBalance:  financetemplates.Money(r.Context(), locale, sum.RealizedBalanceCents),
		},
		Flash:      flash,
		Pagination: molecules.PaginationFrom(locale, page, "/finance", q, "page"),
	}
	for _, row := range accounts {
		view := h.accountView(r.Context(), locale, row.Account)
		view.IsQuota = row.IsQuota
		view.IsLateFee = row.IsLateFee
		if row.IsQuota {
			view.Title = financetemplates.QuotaTitle(locale, row.Period)
		} else if row.IsLateFee {
			view.Title = financetemplates.LateFeeTitle(locale, row.LateFeeOrigin.Origin)
		}
		view.CanReceipt = row.Account.Type == domain.AccountTypeReceivable && view.EffectiveStatus == domain.StatusSettled
		data.Accounts = append(data.Accounts, view)
	}
	return data, nil
}

func (h *Handler) accountView(ctx context.Context, locale i18n.Language, a domain.FinancialAccount) financetemplates.AccountView {
	view := financetemplates.AccountView{
		ID:              a.ID,
		Type:            a.Type,
		Title:           a.Title,
		CategoryLabel:   financetemplates.CategoryLabel(locale, a.Type, a.Category),
		Amount:          financetemplates.Money(ctx, locale, a.AmountCents),
		DueDate:         i18n.FormatDate(locale, a.DueDate),
		EffectiveStatus: a.EffectiveStatus(h.now()),
		StatusLabel:     financetemplates.StatusLabel(locale, a.EffectiveStatus(h.now())),
		SupplierPayee:   a.SupplierPayee,
		HasReceipt:      a.ReceiptPath != "",
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
	case errors.Is(err, services.ErrQuotaManaged):
		httpx.RenderError(w, r, http.StatusConflict, i18n.T(locale, "quotas.error.quota_managed"))
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
	view := h.accountView(r.Context(), locale, account)
	data := financetemplates.SettleData{
		Shell:   shell,
		Locale:  locale,
		CSRF:    sess.CSRFToken,
		Account: view,
		Error:   errorMsg,
	}
	on := h.settleDate(r)
	data.Date = on.Format("2006-01-02")
	if period, ok := h.quotaPeriod(r.Context(), account); ok {
		data.Account.Title = financetemplates.QuotaTitle(locale, period)
		// A quota charge settles with a late fee decision: show the breakdown for
		// the chosen date (today on first load).
		data.IsQuota = h.deps.Quotas != nil
		if data.IsQuota {
			data.LateFee = h.lateFeeFragment(r, account.ID, on)
		}
	} else if origin, ok := h.lateFeeOrigin(r.Context(), account.ID); ok {
		data.Account.Title = financetemplates.LateFeeTitle(locale, origin.Origin)
	}
	return data
}

// lateFeeOrigin returns the origin of a late fee receivable. Any failure or a
// non-late-fee account reports false and the stored title is kept.
func (h *Handler) lateFeeOrigin(ctx context.Context, accountID string) (domain.LateFeeOrigin, bool) {
	origin, ok, err := h.deps.Finance.LateFeeOrigin(ctx, accountID)
	if err != nil {
		log.Printf("finance: late fee origin of %s: %v", accountID, err)
		return domain.LateFeeOrigin{}, false
	}
	return origin, ok
}

// quotaPeriod returns the quota metadata of a receivable so the generic pages
// can show the localised quota title. The account was already loaded for the
// session's condominium, so a single-account lookup is enough. Any failure or a
// non-quota account reports false and the stored title is kept.
func (h *Handler) quotaPeriod(ctx context.Context, a domain.FinancialAccount) (domain.QuotaPeriod, bool) {
	if a.Type != domain.AccountTypeReceivable {
		return domain.QuotaPeriod{}, false
	}
	p, ok, err := h.deps.Finance.QuotaPeriod(ctx, a.ID)
	if err != nil {
		log.Printf("finance settle: quota period of %s: %v", a.ID, err)
		return domain.QuotaPeriod{}, false
	}
	return p, ok
}

func (h *Handler) cancelData(r *http.Request, account domain.FinancialAccount, errorMsg string) financetemplates.CancelData {
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	shell, _ := httpx.ShellData(r, h.deps.Roles, "/finance")
	view := h.accountView(r.Context(), locale, account)
	if origin, ok := h.lateFeeOrigin(r.Context(), account.ID); ok {
		view.IsLateFee = true
		view.Title = financetemplates.LateFeeTitle(locale, origin.Origin)
	}
	return financetemplates.CancelData{
		Shell:   shell,
		Locale:  locale,
		CSRF:    sess.CSRFToken,
		Account: view,
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
	case errors.Is(err, services.ErrQuotaManaged):
		httpx.RenderError(w, r, http.StatusConflict, i18n.T(locale, "quotas.error.quota_managed"))
	default:
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
	}
}

// refuseQuota answers 409 and reports true when the account is a quota
// receivable, which the generic screens must not edit or cancel (FR-16). With
// lateFee set it also refuses a late fee receivable, which cannot be edited
// (FR-8) but can be canceled, so cancel passes false.
func (h *Handler) refuseQuota(w http.ResponseWriter, r *http.Request, accountID string, lateFee bool) bool {
	quota, err := h.deps.Finance.IsQuotaAccount(r.Context(), accountID)
	if err == nil && !quota && lateFee {
		quota, err = h.deps.Finance.IsLateFeeAccount(r.Context(), accountID)
	}
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(i18n.LanguageFrom(r.Context()), "finance.error.load"))
		return true
	}
	if quota {
		httpx.RenderError(w, r, http.StatusConflict, i18n.T(i18n.LanguageFrom(r.Context()), "quotas.error.quota_managed"))
		return true
	}
	return false
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
