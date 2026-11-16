package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/permille"
)

// Extraordinary quota limits.
const (
	maxExtraordinaryDescription = 120
	maxInstallments             = 24
)

// Division is how an extraordinary quota is split among the units.
type Division string

// Divisions (FR-9).
const (
	DivisionShare Division = "share" // by permilagem
	DivisionEqual Division = "equal" // equal parts
)

// Valid reports whether d is a known division.
func (d Division) Valid() bool { return d == DivisionShare || d == DivisionEqual }

// ExtraordinaryInput is the syndic's input of an extraordinary quota.
type ExtraordinaryInput struct {
	Description  string
	TotalCents   int64
	Division     Division
	Installments int
	// FirstDueDate is the due date of installment 1; its day must be 1..28.
	FirstDueDate time.Time
}

// NewExtraordinary is the extraordinary quota header to insert.
type NewExtraordinary struct {
	CondominiumID string
	Description   string
	TotalCents    int64
	Division      Division
	Installments  int
	FirstDueDate  time.Time
	CreatedBy     string
}

// NewInstallment is one installment issuance to insert (reserve is always 0).
// Charges has only the units with a positive amount.
type NewInstallment struct {
	Number     int // 1..N
	TotalCents int64
	DueDate    time.Time
	Charges    []NewCharge
}

// ExtraordinaryInstallment is one installment of a preview.
type ExtraordinaryInstallment struct {
	Number     int
	DueDate    time.Time
	TotalCents int64
}

// ExtraordinaryUnit is the preview of one unit: its amount in each installment.
type ExtraordinaryUnit struct {
	UnitID   string
	Code     string
	SharePPM int
	// PerInstallmentCents has one entry per installment, in order (0 = no receivable).
	PerInstallmentCents []int64
	TotalCents          int64
}

// ExtraordinaryPreview is the preview shown before confirming (FR-9).
type ExtraordinaryPreview struct {
	Description  string
	TotalCents   int64
	Division     Division
	Installments []ExtraordinaryInstallment
	// Units has the units with a positive share.
	Units  []ExtraordinaryUnit
	SumPPM int
}

// ExtraordinarySummary is an extraordinary quota in the list (FR-12).
type ExtraordinarySummary struct {
	ID            string
	CondominiumID string
	Description   string
	TotalCents    int64
	Division      Division
	Installments  int
	FirstDueDate  time.Time
	Status        domain.IssuanceStatus // issued | canceled
	CreatedBy     string
	CreatedAt     time.Time
	CanceledAt    *time.Time
	// ChargeCount is the number of receivables over all installments;
	// SettledCount how many are settled.
	ChargeCount  int
	SettledCount int
	// SettledInstallments counts the installments whose receivables are all settled.
	SettledInstallments int
}

// ExtraordinaryDetail is an extraordinary quota with its installments.
type ExtraordinaryDetail struct {
	ExtraordinarySummary
	// Issuances are the installment issuances by number (PeriodIndex).
	Issuances []IssuanceSummary
}

// DueDateOf is the due date of installment n.
func (e ExtraordinarySummary) DueDateOf(n int) time.Time {
	return domain.InstallmentDueDate(e.FirstDueDate, n)
}

func (s *QuotaService) auditLogged(ctx context.Context, actorID string, ev model.AuditEventType, details map[string]any) {
	if err := s.audit(ctx, actorID, ev, details); err != nil {
		slog.Error("finance: record extraordinary quota audit", "event", string(ev), "error", err)
	}
}

// computeExtraordinary validates the input and computes the preview (pure).
func computeExtraordinary(in ExtraordinaryInput, shares []domain.ShareInput) (ExtraordinaryPreview, error) {
	in.Description = strings.TrimSpace(in.Description)
	p := ExtraordinaryPreview{Description: in.Description, TotalCents: in.TotalCents, Division: in.Division, SumPPM: sumShares(shares)}
	if in.Description == "" || utf8.RuneCountInString(in.Description) > maxExtraordinaryDescription ||
		in.TotalCents <= 0 || !in.Division.Valid() || in.FirstDueDate.IsZero() {
		return p, domain.ErrInvalidExtraordinary
	}
	if d := in.FirstDueDate.Day(); d < 1 || d > 28 {
		return p, domain.ErrInvalidExtraordinary
	}
	if in.Installments < 1 || in.Installments > maxInstallments || in.TotalCents < int64(in.Installments) {
		return p, domain.ErrInvalidInstallments
	}
	if in.Division == DivisionShare && p.SumPPM != permille.Total {
		return p, SharesError{SumPPM: p.SumPPM}
	}
	if p.SumPPM == 0 {
		return p, domain.ErrNoChargeableUnits
	}
	parts := domain.SplitInstallments(in.TotalCents, in.Installments)
	for _, sh := range shares {
		if sh.SharePPM > 0 {
			p.Units = append(p.Units, ExtraordinaryUnit{UnitID: sh.UnitID, Code: sh.Code, SharePPM: sh.SharePPM,
				PerInstallmentCents: make([]int64, in.Installments)})
		}
	}
	for n, amount := range parts {
		p.Installments = append(p.Installments, ExtraordinaryInstallment{
			Number: n + 1, DueDate: domain.InstallmentDueDate(in.FirstDueDate, n+1), TotalCents: amount,
		})
		var alloc []domain.Allocation
		if in.Division == DivisionShare {
			alloc = domain.AllocateByShare(amount, 0, shares)
		} else {
			alloc = domain.AllocateEqual(amount, shares)
		}
		for u, a := range chargeable(alloc) {
			p.Units[u].PerInstallmentCents[n] = a.TotalCents()
			p.Units[u].TotalCents += a.TotalCents()
		}
	}
	return p, nil
}

// PreviewExtraordinary previews an extraordinary quota (FR-9, FR-10).
func (s *QuotaService) PreviewExtraordinary(ctx context.Context, condoID string, in ExtraordinaryInput) (ExtraordinaryPreview, error) {
	shares, err := s.unitShares(ctx, condoID)
	if err != nil {
		return ExtraordinaryPreview{}, err
	}
	return computeExtraordinary(in, shares)
}

// CreateExtraordinary issues every installment atomically (FR-11, NFR-2): one
// pending receivable per unit with a positive amount, category extraordinary
// income. It returns the extraordinary quota id.
func (s *QuotaService) CreateExtraordinary(ctx context.Context, actorID, condoID string, in ExtraordinaryInput) (string, error) {
	shares, err := s.unitShares(ctx, condoID)
	if err != nil {
		return "", err
	}
	pv, err := computeExtraordinary(in, shares)
	if err != nil {
		return "", err
	}
	n := len(pv.Installments)
	inst := make([]NewInstallment, n)
	for i, pi := range pv.Installments {
		inst[i] = NewInstallment{Number: pi.Number, TotalCents: pi.TotalCents, DueDate: pi.DueDate}
		for _, u := range pv.Units {
			amount := u.PerInstallmentCents[i]
			if amount <= 0 {
				continue
			}
			inst[i].Charges = append(inst[i].Charges, NewCharge{
				UnitID: u.UnitID, SharePPM: u.SharePPM,
				// Neutral fallback; views render the localised title from the quota period.
				Title:       fmt.Sprintf("Quota extraordinária %s %d/%d %s", pv.Description, pi.Number, n, u.Code),
				AmountCents: amount, BaseCents: amount, ReserveCents: 0,
				DueDate: pi.DueDate, CreatedBy: actorID,
			})
		}
	}
	id, err := s.Store.CreateExtraordinary(ctx, NewExtraordinary{
		CondominiumID: condoID, Description: pv.Description, TotalCents: pv.TotalCents, Division: pv.Division,
		Installments: n, FirstDueDate: pv.Installments[0].DueDate, CreatedBy: actorID,
	}, inst)
	if err != nil {
		return "", err
	}
	s.auditLogged(ctx, actorID, model.AuditExtraordinaryQuotaCreated, map[string]any{
		"extraordinary_id": id, "condominium_id": condoID, "description": pv.Description,
		"total_cents": pv.TotalCents, "division": string(pv.Division), "installments": n,
	})
	return id, nil
}

// ListExtraordinaries lists the condominium's extraordinary quotas (FR-12).
func (s *QuotaService) ListExtraordinaries(ctx context.Context, condoID string) ([]ExtraordinarySummary, error) {
	return s.Store.ListExtraordinaries(ctx, condoID)
}

// GetExtraordinary returns one extraordinary quota with its installments.
func (s *QuotaService) GetExtraordinary(ctx context.Context, condoID, id string) (ExtraordinaryDetail, error) {
	return s.Store.GetExtraordinary(ctx, condoID, id)
}

// CancelExtraordinary cancels a whole extraordinary quota when none of its
// receivables is settled (FR-12, NFR-2). It locks only the parent row.
func (s *QuotaService) CancelExtraordinary(ctx context.Context, actorID, condoID, id string) error {
	var ex ExtraordinarySummary
	err := s.Store.WithLockedExtraordinary(ctx, condoID, id, func(_ context.Context, e LockedExtraordinary) error {
		ex = e.Extraordinary()
		if ex.Status != domain.IssuanceIssued {
			return ErrIssuanceNotActive
		}
		affected, total, err := e.CancelPendingCharges()
		if err != nil {
			return err
		}
		if affected != total {
			return domain.ErrExtraordinaryHasSettled // rolls the transaction back
		}
		return e.MarkCanceled(s.now())
	})
	if err != nil {
		return err
	}
	s.auditLogged(ctx, actorID, model.AuditExtraordinaryQuotaCanceled, map[string]any{
		"extraordinary_id": id, "condominium_id": condoID, "description": ex.Description,
	})
	return nil
}
