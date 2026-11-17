// Package services implements the finance use cases: account lifecycle,
// listing/filtering, dashboard summary, and resident views.
package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/pagination"
)

// Service errors surfaced to handlers.
var (
	// ErrInvalidInput covers missing required fields and invalid account type.
	ErrInvalidInput = errors.New("invalid account input")
	// ErrInvalidAmount aliases the domain amount error.
	ErrInvalidAmount = domain.ErrInvalidAmount
	// ErrInvalidCategory aliases the domain category error.
	ErrInvalidCategory = domain.ErrInvalidCategory
	// ErrInvalidDueDate aliases the domain due-date error.
	ErrInvalidDueDate = domain.ErrInvalidDueDate
	// ErrSettlementDateRequired aliases the domain settlement-date error.
	ErrSettlementDateRequired = domain.ErrSettlementDateRequired
	// ErrInvalidUnit is returned when the unit does not belong to the condominium.
	ErrInvalidUnit = errors.New("invalid unit")
	// ErrNotEditable aliases the domain read-only guard.
	ErrNotEditable = domain.ErrNotEditable
	// ErrNoUnit is returned when a resident has no active unit.
	ErrNoUnit = errors.New("resident has no active unit")
	// ErrNotFound aliases the domain-level not-found sentinel.
	ErrNotFound = model.ErrNotFound
	// ErrQuotaManaged is returned when editing or canceling a receivable that
	// belongs to a quota issuance (FR-16); it is changed only through its issuance.
	ErrQuotaManaged = errors.New("account is managed by a quota issuance")
	// ErrForbidden is returned when the viewer may not see the requested resource.
	ErrForbidden = errors.New("forbidden")
)

// AccountRow is an account annotated for the generic list.
type AccountRow struct {
	Account domain.FinancialAccount
	// IsQuota is true when the account is a quota receivable (FR-16).
	IsQuota bool
	// Period is the quota period and unit, set when IsQuota (display-time i18n).
	Period domain.QuotaPeriod
	// IsLateFee is true for a late fee receivable; LateFeeOrigin is then the
	// quota it comes from and its stored breakdown (R2).
	IsLateFee     bool
	LateFeeOrigin domain.LateFeeOrigin
}

// AccountInput is a validated create/edit payload. Amount and dates are the
// raw form strings; parsing happens in the service.
type AccountInput struct {
	Type           domain.AccountType
	Title          string
	Category       domain.Category
	Amount         string
	DueDate        string
	Status         domain.AccountStatus
	SettlementDate string
	SupplierPayee  string
	ReceiptPath    string
	ReceiptName    string
	UnitID         string
	PaymentCode    string
}

// FinanceService implements the finance use cases.
type FinanceService struct {
	Accounts  AccountStore
	Summaries SummaryStore
	Units     UnitStore
	Audit     AuditRecorder
	Overviews OverviewStore
	// QuotaLinks is optional; nil means no account is quota-managed.
	QuotaLinks QuotaLinks
	// LateFeeLinks is optional; nil means no account is a late fee.
	LateFeeLinks LateFeeLinks
	Now          func() time.Time
}

// IsLateFeeAccount reports whether the account is a late fee receivable, so
// the generic edit screen can refuse it up front (nil-safe).
func (s *FinanceService) IsLateFeeAccount(ctx context.Context, accountID string) (bool, error) {
	if s.LateFeeLinks == nil {
		return false, nil
	}
	m, err := s.LateFeeLinks.LateFeeOrigins(ctx, []string{accountID})
	if err != nil {
		return false, err
	}
	_, ok := m[accountID]
	return ok, nil
}

// LateFeeOrigin returns the origin quota and breakdown of one late fee account;
// ok is false when the account is not a late fee or LateFeeLinks is not wired.
func (s *FinanceService) LateFeeOrigin(ctx context.Context, accountID string) (domain.LateFeeOrigin, bool, error) {
	if s.LateFeeLinks == nil {
		return domain.LateFeeOrigin{}, false, nil
	}
	m, err := s.LateFeeLinks.LateFeeOrigins(ctx, []string{accountID})
	if err != nil {
		return domain.LateFeeOrigin{}, false, err
	}
	o, ok := m[accountID]
	return o, ok, nil
}

// QuotaPeriod returns the quota metadata of one account. ok is false when the
// account is not linked to a quota or when QuotaLinks is not wired (nil-safe).
// Callers pass an account they already loaded for their condominium.
func (s *FinanceService) QuotaPeriod(ctx context.Context, accountID string) (domain.QuotaPeriod, bool, error) {
	if s.QuotaLinks == nil {
		return domain.QuotaPeriod{}, false, nil
	}
	m, err := s.QuotaLinks.QuotaPeriods(ctx, []string{accountID})
	if err != nil {
		return domain.QuotaPeriod{}, false, err
	}
	p, ok := m[accountID]
	return p, ok, nil
}

// isQuota reports whether the account is linked to a quota (nil-safe).
func (s *FinanceService) isQuota(ctx context.Context, accountID string) (bool, error) {
	_, ok, err := s.QuotaPeriod(ctx, accountID)
	return ok, err
}

// IsQuotaAccount reports whether the account is a quota receivable, so the
// generic edit/cancel screens can refuse it up front (FR-16).
func (s *FinanceService) IsQuotaAccount(ctx context.Context, accountID string) (bool, error) {
	return s.isQuota(ctx, accountID)
}

// ListAccountsWithFlags is ListAccounts annotated with IsQuota using one batch lookup.
func (s *FinanceService) ListAccountsWithFlags(ctx context.Context, condominiumID string, f AccountFilter) ([]AccountRow, error) {
	accs, err := s.Accounts.List(ctx, condominiumID, f)
	if err != nil {
		return nil, err
	}
	return s.annotate(ctx, accs)
}

// ListAccountsPageWithFlags pages the accounts in the database first, then
// annotates only the page's rows with the quota and late fee flags.
func (s *FinanceService) ListAccountsPageWithFlags(ctx context.Context, condominiumID string, f AccountFilter, requested int) ([]AccountRow, pagination.Page, error) {
	accs, page, err := s.Accounts.ListPage(ctx, condominiumID, f, requested)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	rows, err := s.annotate(ctx, accs)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return rows, page, nil
}

// annotate adds the quota and late fee flags to accs with one batch lookup each.
func (s *FinanceService) annotate(ctx context.Context, accs []domain.FinancialAccount) ([]AccountRow, error) {
	var err error
	rows := make([]AccountRow, len(accs))
	var quota map[string]domain.QuotaPeriod
	if s.QuotaLinks != nil && len(accs) > 0 {
		ids := make([]string, len(accs))
		for i, a := range accs {
			ids[i] = a.ID
		}
		if quota, err = s.QuotaLinks.QuotaPeriods(ctx, ids); err != nil {
			return nil, err
		}
	}
	var lateFees map[string]domain.LateFeeOrigin
	if s.LateFeeLinks != nil && len(accs) > 0 {
		ids := make([]string, len(accs))
		for i, a := range accs {
			ids[i] = a.ID
		}
		if lateFees, err = s.LateFeeLinks.LateFeeOrigins(ctx, ids); err != nil {
			return nil, err
		}
	}
	for i, a := range accs {
		p, ok := quota[a.ID]
		o, late := lateFees[a.ID]
		rows[i] = AccountRow{Account: a, IsQuota: ok, Period: p, IsLateFee: late, LateFeeOrigin: o}
	}
	return rows, nil
}

// CreateAccount validates and stores a new account. Initial status is
// pending by default; creating as settled requires the transaction date
// (clarification session 2026-08-23).
func (s *FinanceService) CreateAccount(ctx context.Context, actorID, condominiumID string, in AccountInput) (domain.FinancialAccount, error) {
	acc, err := s.validateAndBuild(ctx, condominiumID, "", in)
	if err != nil {
		return domain.FinancialAccount{}, err
	}
	acc.CreatedBy = actorID
	id, err := s.Accounts.Create(ctx, acc)
	if err != nil {
		return domain.FinancialAccount{}, err
	}
	acc.ID = id
	uid := actorID
	if err := s.Audit.RecordEvent(ctx, &uid, model.AuditAccountCreated,
		map[string]any{"account_id": id, "account_type": string(acc.Type), "condominium_id": condominiumID}); err != nil {
		return domain.FinancialAccount{}, err
	}
	return acc, nil
}

// EditAccount updates an editable (pending/overdue) account's fields.
func (s *FinanceService) EditAccount(ctx context.Context, actorID, accountID, condominiumID string, in AccountInput) error {
	existing, err := s.Accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if existing.CondominiumID != condominiumID {
		return ErrNotFound
	}
	if quota, err := s.isQuota(ctx, accountID); err != nil {
		return err
	} else if quota {
		return ErrQuotaManaged
	}
	// A late fee is recorded as calculated: it can be canceled, never edited.
	if late, err := s.IsLateFeeAccount(ctx, accountID); err != nil {
		return err
	} else if late {
		return ErrQuotaManaged
	}
	if !existing.CanEdit() {
		return ErrNotEditable
	}
	acc, err := s.validateAndBuild(ctx, condominiumID, accountID, in)
	if err != nil {
		return err
	}
	acc.ID = accountID
	acc.CondominiumID = condominiumID
	if err := s.Accounts.Update(ctx, acc); err != nil {
		return err
	}
	uid := actorID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditAccountEdited,
		map[string]any{"account_id": accountID, "condominium_id": condominiumID})
}

// SettleAccount marks an account as settled, requiring the transaction date
// (FR-010).
func (s *FinanceService) SettleAccount(ctx context.Context, actorID, accountID, condominiumID, settlementDate string) error {
	existing, err := s.Accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if existing.CondominiumID != condominiumID {
		return ErrNotFound
	}
	// Quota charges settle only through QuotaService.SettleQuota (late fee decision).
	if quota, err := s.isQuota(ctx, accountID); err != nil {
		return err
	} else if quota {
		return ErrQuotaManaged
	}
	if !existing.CanSettle() {
		return ErrNotEditable
	}
	date, err := domain.ParseSettlementDate(settlementDate)
	if err != nil {
		return err
	}
	if err := s.Accounts.SetStatus(ctx, accountID, condominiumID, domain.StatusSettled, &date); err != nil {
		return err
	}
	uid := actorID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditAccountSettled,
		map[string]any{"account_id": accountID, "condominium_id": condominiumID})
}

// CancelAccount marks a pending/overdue account as canceled (FR-019).
func (s *FinanceService) CancelAccount(ctx context.Context, actorID, accountID, condominiumID string) error {
	existing, err := s.Accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if existing.CondominiumID != condominiumID {
		return ErrNotFound
	}
	if quota, err := s.isQuota(ctx, accountID); err != nil {
		return err
	} else if quota {
		return ErrQuotaManaged
	}
	if !existing.CanCancel() {
		return ErrNotEditable
	}
	origin, late, err := s.LateFeeOrigin(ctx, accountID)
	if err != nil {
		return err
	}
	if err := s.Accounts.SetStatus(ctx, accountID, condominiumID, domain.StatusCanceled, nil); err != nil {
		return err
	}
	uid := actorID
	if err := s.Audit.RecordEvent(ctx, &uid, model.AuditAccountCanceled,
		map[string]any{"account_id": accountID, "condominium_id": condominiumID}); err != nil {
		return err
	}
	if late {
		// Later waiver of an applied late fee (FR-8, FR-19). The cancel is
		// already committed, so an audit failure is logged, not returned.
		if err := s.Audit.RecordEvent(ctx, &uid, model.AuditLateFeeWaived, map[string]any{
			"late_fee_account_id": accountID, "condominium_id": condominiumID, "stage": "cancel",
			"amount_cents": existing.AmountCents, "fine_cents": origin.Detail.FineCents,
			"interest_cents": origin.Detail.InterestCents, "days_late": origin.Detail.DaysLate,
		}); err != nil {
			slog.Error("finance: record late fee waiver audit", "error", err)
		}
	}
	return nil
}

// ListAccounts returns the condominium's accounts matching the filter.
func (s *FinanceService) ListAccounts(ctx context.Context, condominiumID string, f AccountFilter) ([]domain.FinancialAccount, error) {
	return s.Accounts.List(ctx, condominiumID, f)
}

// Summary returns the dashboard projection for the month (FR-013).
func (s *FinanceService) Summary(ctx context.Context, condominiumID string, month time.Time) (MonthSummary, error) {
	return s.Summaries.SummaryForMonth(ctx, condominiumID, month)
}

// validateAndBuild validates an AccountInput and produces a
// domain.FinancialAccount. On edit the account type is taken from existingID's
// account so the type can never change.
func (s *FinanceService) validateAndBuild(ctx context.Context, condominiumID, existingID string, in AccountInput) (domain.FinancialAccount, error) {
	accType := in.Type
	if existingID != "" {
		existing, err := s.Accounts.GetByID(ctx, existingID)
		if err != nil {
			return domain.FinancialAccount{}, err
		}
		accType = existing.Type
	}
	if !accType.Valid() {
		return domain.FinancialAccount{}, ErrInvalidInput
	}
	if err := domain.ValidateTitle(in.Title); err != nil {
		return domain.FinancialAccount{}, ErrInvalidInput
	}
	if !in.Category.ValidFor(accType) {
		return domain.FinancialAccount{}, ErrInvalidCategory
	}
	amount, err := domain.ParseAmountCents(in.Amount)
	if err != nil {
		return domain.FinancialAccount{}, ErrInvalidAmount
	}
	dueDate, err := domain.ParseDate(in.DueDate)
	if err != nil {
		return domain.FinancialAccount{}, ErrInvalidDueDate
	}

	status := in.Status
	if status == "" {
		status = domain.StatusPending
	}
	if existingID != "" {
		// Status is only changed through Settle/Cancel, never through edit.
		existing, _ := s.Accounts.GetByID(ctx, existingID)
		status = existing.Status
	}
	if !status.ValidStored() {
		return domain.FinancialAccount{}, ErrInvalidInput
	}
	var settlement *time.Time
	switch {
	case status == domain.StatusSettled:
		date, err := domain.ParseSettlementDate(in.SettlementDate)
		if err != nil {
			return domain.FinancialAccount{}, err
		}
		settlement = &date
	case in.SettlementDate != "":
		return domain.FinancialAccount{}, ErrInvalidInput
	}

	acc := domain.FinancialAccount{
		CondominiumID:  condominiumID,
		Type:           accType,
		Title:          strings.TrimSpace(in.Title),
		Category:       in.Category,
		AmountCents:    amount,
		DueDate:        dueDate,
		Status:         status,
		SettlementDate: settlement,
	}

	if accType == domain.AccountTypePayable {
		if err := domain.ValidateSupplier(in.SupplierPayee); err != nil {
			return domain.FinancialAccount{}, ErrInvalidInput
		}
		if in.UnitID != "" || in.PaymentCode != "" {
			return domain.FinancialAccount{}, ErrInvalidInput
		}
		acc.SupplierPayee = strings.TrimSpace(in.SupplierPayee)
		acc.ReceiptPath = in.ReceiptPath
		acc.ReceiptName = in.ReceiptName
	} else {
		if err := domain.ValidatePaymentCode(in.PaymentCode); err != nil {
			return domain.FinancialAccount{}, ErrInvalidInput
		}
		if in.SupplierPayee != "" || in.ReceiptPath != "" || in.ReceiptName != "" {
			return domain.FinancialAccount{}, ErrInvalidInput
		}
		if in.UnitID != "" {
			ok, err := s.unitInCondominium(ctx, in.UnitID, condominiumID)
			if err != nil {
				return domain.FinancialAccount{}, err
			}
			if !ok {
				return domain.FinancialAccount{}, ErrInvalidUnit
			}
			acc.UnitID = in.UnitID
		}
		acc.PaymentCode = strings.TrimSpace(in.PaymentCode)
	}
	return acc, nil
}

func (s *FinanceService) unitInCondominium(ctx context.Context, unitID, condominiumID string) (bool, error) {
	units, err := s.Units.ListUnitsForCondominium(ctx, condominiumID)
	if err != nil {
		return false, err
	}
	for _, u := range units {
		if u.ID == unitID {
			return true, nil
		}
	}
	return false, nil
}
