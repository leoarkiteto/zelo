package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// Late fee (mora) decisions taken when a late quota is settled (R6).
const (
	LateFeeApply = "apply"
	LateFeeWaive = "waive"
)

// ErrLateFeeDecisionRequired is returned by SettleQuota when the late fee of
// the chosen settlement date is positive and the caller gave no decision.
var ErrLateFeeDecisionRequired = errors.New("late fee decision required")

// LateFeeQuote is the late fee a quota would owe when settled on a date, with
// the effective rates used (the BRL clamp already applied).
type LateFeeQuote struct {
	domain.LateFee
	FeeBP      int
	InterestBP int
	GraceDays  int
}

// lateRates returns the rates in force now: zero when LateFees is not wired, and
// the fine clamped to model.MaxBRLLateFeeBP for a BRL condominium (R5).
func (s *QuotaService) lateRates(ctx context.Context, condoID string) (domain.LateFeeRates, error) {
	if s.LateFees == nil {
		return domain.LateFeeRates{}, nil
	}
	ls, err := s.LateFees(ctx, condoID)
	if err != nil {
		return domain.LateFeeRates{}, err
	}
	r := domain.LateFeeRates{FeeBP: ls.FeeBP, InterestBP: ls.InterestBP, GraceDays: ls.GraceDays}
	if s.Currency != nil {
		cur, err := s.Currency(ctx, condoID)
		if err != nil {
			return domain.LateFeeRates{}, err
		}
		if cur == money.BRL && r.FeeBP > model.MaxBRLLateFeeBP {
			r.FeeBP = model.MaxBRLLateFeeBP
		}
	}
	return r, nil
}

func quoteOf(a QuotaAccount, settledOn time.Time, r domain.LateFeeRates) LateFeeQuote {
	return LateFeeQuote{
		LateFee: domain.ComputeLateFee(a.AmountCents, a.DueDate, settledOn, r),
		FeeBP:   r.FeeBP, InterestBP: r.InterestBP, GraceDays: r.GraceDays,
	}
}

// LateFeePreview returns the late fee a quota would owe if settled on settledOn,
// with the settings in force now. The result is zero when the account is not a
// pending quota charge or is not late (never a late fee on a late fee, FR-5).
func (s *QuotaService) LateFeePreview(ctx context.Context, condoID, accountID string, settledOn time.Time) (LateFeeQuote, error) {
	a, err := s.Store.QuotaAccount(ctx, condoID, accountID)
	if err != nil {
		return LateFeeQuote{}, err
	}
	if !a.IsQuota || a.Status != domain.StatusPending {
		return LateFeeQuote{}, nil
	}
	r, err := s.lateRates(ctx, condoID)
	if err != nil {
		return LateFeeQuote{}, err
	}
	return quoteOf(a, settledOn, r), nil
}

// SettleQuota is the only way to settle a quota charge, ordinary or installment
// (R6). In one transaction holding the account row lock it re-checks that the
// account is a pending quota charge, recomputes the late fee with the settings
// resolved just before the lock (never queried while the row is held), and
// settles it. A positive late fee needs a decision: "apply"
// also creates the late fee receivable linked to the quota, "waive" audits the
// waiver. Two concurrent settlements produce at most one settlement and one
// late fee (the row lock plus UNIQUE quota_account_id, NFR-2); the loser gets
// ErrNotEditable. Audit failures after the commit are logged, not returned.
func (s *QuotaService) SettleQuota(ctx context.Context, actorID, condoID, accountID string, settledOn time.Time, decision string) error {
	if decision != "" && decision != LateFeeApply && decision != LateFeeWaive {
		return ErrInvalidInput
	}
	if settledOn.IsZero() {
		return ErrSettlementDateRequired
	}
	settledOn = utcDate(settledOn)
	// Resolved before taking the row lock: these are pool queries and must not
	// run (or wait for a connection) while the account row is held.
	r, err := s.lateRates(ctx, condoID)
	if err != nil {
		return err
	}
	var (
		quote     LateFeeQuote
		lateFeeID string
	)
	err = s.Store.WithLockedQuotaAccount(ctx, condoID, accountID, func(_ context.Context, l LockedQuotaAccount) error {
		a := l.Account()
		if !a.IsQuota {
			return ErrNotFound // not a quota charge (plain receivables and late fees settle generically)
		}
		if a.Status != domain.StatusPending {
			return ErrNotEditable
		}
		quote = quoteOf(a, settledOn, r)
		if quote.TotalCents > 0 && decision == "" {
			return ErrLateFeeDecisionRequired
		}
		if err := l.Settle(settledOn); err != nil {
			return err
		}
		if quote.TotalCents > 0 && decision == LateFeeApply {
			lateFeeID, err = l.InsertLateFee(NewLateFee{
				// Neutral fallback; views render the localised title from the origin quota.
				Title: "Multa e juros — " + a.Title, DueDate: settledOn, CreatedBy: actorID,
				Detail: domain.LateFeeDetail{
					FineCents: quote.FineCents, InterestCents: quote.InterestCents, DaysLate: quote.DaysLate,
					FeeBP: quote.FeeBP, InterestBP: quote.InterestBP, GraceDays: quote.GraceDays,
				},
			})
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.auditLoggedLateFee(ctx, actorID, model.AuditAccountSettled, map[string]any{
		"account_id": accountID, "condominium_id": condoID, "settlement_date": settledOn.Format("2006-01-02"),
	})
	if quote.TotalCents > 0 {
		details := map[string]any{
			"quota_account_id": accountID, "condominium_id": condoID, "amount_cents": quote.TotalCents,
			"fine_cents": quote.FineCents, "interest_cents": quote.InterestCents, "days_late": quote.DaysLate,
			"fee_bp": quote.FeeBP, "interest_bp": quote.InterestBP, "grace_days": quote.GraceDays,
		}
		switch decision {
		case LateFeeApply:
			details["late_fee_account_id"] = lateFeeID
			s.auditLoggedLateFee(ctx, actorID, model.AuditLateFeeApplied, details)
		case LateFeeWaive:
			details["stage"] = "settlement"
			s.auditLoggedLateFee(ctx, actorID, model.AuditLateFeeWaived, details)
		}
	}
	return nil
}

func (s *QuotaService) auditLoggedLateFee(ctx context.Context, actorID string, ev model.AuditEventType, details map[string]any) {
	if err := s.audit(ctx, actorID, ev, details); err != nil {
		slog.Error("finance: record late fee audit", "event", string(ev), "error", err)
	}
}

// DebtorOccupant is an active occupant of a debtor unit.
type DebtorOccupant struct {
	Email string
	Type  model.OccupancyType
}

// DebtorRow is the store projection of one unit with overdue receivables.
type DebtorRow struct {
	UnitID            string
	UnitCode          string
	Occupants         []DebtorOccupant
	OverdueCount      int
	OverdueTotalCents int64
	OldestDue         time.Time
}

// OverdueQuota is a pending quota charge due before today.
type OverdueQuota struct {
	UnitID      string
	AmountCents int64
	DueDate     time.Time
}

// Debtor is a unit of the debtors list (FR-1, FR-2).
type Debtor struct {
	DebtorRow
	// DaysLate counts the days from the oldest overdue due date to today.
	DaysLate int
	// EstimatedLateFeeCents is the late fee of the unit's overdue quotas
	// computed for today with the current settings; informative, syndic only.
	EstimatedLateFeeCents int64
}

// Debtors lists the units with overdue receivables, biggest total first, with
// the estimated late fee of their overdue quotas up to today. It runs two
// set-based queries regardless of the number of units (NFR-5).
func (s *QuotaService) Debtors(ctx context.Context, condoID string, today time.Time) ([]Debtor, error) {
	today = utcDate(today)
	rows, err := s.Store.DebtorRows(ctx, condoID, today)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	quotas, err := s.Store.OverdueQuotaRows(ctx, condoID, today)
	if err != nil {
		return nil, err
	}
	r, err := s.lateRates(ctx, condoID)
	if err != nil {
		return nil, err
	}
	estimate := make(map[string]int64, len(rows))
	for _, q := range quotas {
		estimate[q.UnitID] += domain.ComputeLateFee(q.AmountCents, q.DueDate, today, r).TotalCents
	}
	out := make([]Debtor, len(rows))
	for i, row := range rows {
		out[i] = Debtor{
			DebtorRow:             row,
			DaysLate:              int(today.Sub(utcDate(row.OldestDue)) / (24 * time.Hour)),
			EstimatedLateFeeCents: estimate[row.UnitID],
		}
	}
	return out, nil
}

// ReserveTotals is the store projection of the reserve fund parts.
type ReserveTotals struct {
	// SettledCents is the reserve of the settled quota charges, PendingCents of
	// the pending ones (canceled charges count nowhere), SettledYearCents the
	// part settled within the requested calendar year.
	SettledCents     int64
	PendingCents     int64
	SettledYearCents int64
}

// ReserveBalance is the reserve fund balance (FR-17).
type ReserveBalance struct {
	// RealizedCents is the reserve of all settled quotas since the start.
	RealizedCents int64
	// ForecastCents is RealizedCents plus the reserve of the pending quotas.
	ForecastCents int64
	// RealizedYearCents is the reserve settled within the current calendar year.
	RealizedYearCents int64
}

// ReserveBalance returns the reserve fund balance as of today.
func (s *QuotaService) ReserveBalance(ctx context.Context, condoID string, today time.Time) (ReserveBalance, error) {
	t, err := s.Store.ReserveTotals(ctx, condoID, utcDate(today).Year())
	if err != nil {
		return ReserveBalance{}, fmt.Errorf("reserve balance: %w", err)
	}
	return ReserveBalance{
		RealizedCents: t.SettledCents, ForecastCents: t.SettledCents + t.PendingCents, RealizedYearCents: t.SettledYearCents,
	}, nil
}
