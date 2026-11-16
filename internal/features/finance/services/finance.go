// Package services implements the finance use cases: account lifecycle,
// listing/filtering, dashboard summary, and resident views.
package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
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
	Now        func() time.Time
}

// isQuota reports whether the account is linked to a quota (nil-safe).
func (s *FinanceService) isQuota(ctx context.Context, accountID string) (bool, error) {
	if s.QuotaLinks == nil {
		return false, nil
	}
	m, err := s.QuotaLinks.QuotaPeriods(ctx, []string{accountID})
	if err != nil {
		return false, err
	}
	_, ok := m[accountID]
	return ok, nil
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
	for i, a := range accs {
		p, ok := quota[a.ID]
		rows[i] = AccountRow{Account: a, IsQuota: ok, Period: p}
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
	if err := s.Accounts.SetStatus(ctx, accountID, condominiumID, domain.StatusCanceled, nil); err != nil {
		return err
	}
	uid := actorID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditAccountCanceled,
		map[string]any{"account_id": accountID, "condominium_id": condominiumID})
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
