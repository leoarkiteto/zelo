package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/permille"
)

// ErrIssuanceNotActive is returned when canceling an issuance that is not active.
var ErrIssuanceNotActive = errors.New("issuance is not active")

// SharesError is the ErrSharesNot100 refusal carrying the actual share sum so
// the handler can show the difference. errors.Is(err, domain.ErrSharesNot100) holds.
type SharesError struct {
	// SumPPM is the sum of all unit shares in ppm (1,000,000 = 100%).
	SumPPM int
}

func (e SharesError) Error() string {
	return fmt.Sprintf("%v: sum is %d ppm", domain.ErrSharesNot100, e.SumPPM)
}

// Is makes SharesError match domain.ErrSharesNot100.
func (e SharesError) Is(target error) bool { return target == domain.ErrSharesNot100 }

// DiffPPM is the sum minus 100% (negative when shares are missing).
func (e SharesError) DiffPPM() int { return e.SumPPM - permille.Total }

// Budget is an annual quota budget (Orcamento).
type Budget struct {
	ID            string
	CondominiumID string
	Year          int
	Status        domain.BudgetStatus
	Periodicity   domain.Periodicity
	DueDay        int
	ReserveRateBP int
	ApprovedAt    *time.Time
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// BudgetLine is one expense line of a budget.
type BudgetLine struct {
	ID          string
	BudgetID    string
	Description string
	Category    domain.Category // a payable category
	AmountCents int64
	Position    int
}

// BudgetParams are the editable budget parameters.
type BudgetParams struct {
	Periodicity   domain.Periodicity
	DueDay        int
	ReserveRateBP int
}

// BudgetInput creates a budget. A nil ReserveRateBP selects the currency default
// (10% EUR, 0% BRL).
type BudgetInput struct {
	Year          int
	Periodicity   domain.Periodicity
	DueDay        int
	ReserveRateBP *int
}

// LineInput is the payload of a budget line add/edit.
type LineInput struct {
	Description string
	Category    domain.Category
	AmountCents int64
}

// BudgetSummary is a budget in the list.
type BudgetSummary struct {
	Budget
	LineCount  int
	TotalCents int64 // sum of the lines
}

// PeriodQuota is the budget amount of one period.
type PeriodQuota struct {
	Index        int
	Label        string
	DueDate      time.Time
	BaseCents    int64
	ReserveCents int64
	TotalCents   int64
}

// UnitQuotas is the per-period quota preview of one unit.
type UnitQuotas struct {
	UnitID   string
	Code     string
	SharePPM int
	// PerPeriodCents has one entry per period, in order.
	PerPeriodCents []int64
}

// BudgetDetail is a budget with its lines and the computed preview.
type BudgetDetail struct {
	Budget
	Lines      []BudgetLine
	TotalCents int64
	// SumPPM is the sum of the unit shares (the preview is indicative unless it is 1,000,000).
	SumPPM  int
	Periods []PeriodQuota
	Units   []UnitQuotas
}

// NewIssuance is the issuance header to insert.
type NewIssuance struct {
	BudgetID     string
	PeriodIndex  int
	TotalCents   int64 // base + reserve
	BaseCents    int64
	ReserveCents int64
	CreatedBy    string
}

// NewCharge is one quota receivable to insert (category condo_fee, pending).
type NewCharge struct {
	UnitID       string
	SharePPM     int // frozen share (FR-17)
	Title        string
	AmountCents  int64 // base + reserve, > 0
	BaseCents    int64
	ReserveCents int64
	DueDate      time.Time
	CreatedBy    string
}

// IssuanceSummary is an issuance with its settlement progress.
type IssuanceSummary struct {
	ID           string
	BudgetID     string
	Year         int
	Periodicity  domain.Periodicity
	PeriodIndex  int
	Status       domain.IssuanceStatus
	TotalCents   int64
	BaseCents    int64
	ReserveCents int64
	// ChargeCount is the number of quotas; SettledCount how many are settled.
	ChargeCount  int
	SettledCount int
	CreatedAt    time.Time
	CanceledAt   *time.Time
}

// Label is the period label of the issuance ("2026-03", "2026-T2").
func (i IssuanceSummary) Label() string {
	return domain.PeriodLabel(i.Year, i.Periodicity, i.PeriodIndex)
}

// PreviewRow is the quota of one unit in an issuance preview.
type PreviewRow struct {
	UnitID       string
	Code         string
	SharePPM     int
	BaseCents    int64
	ReserveCents int64
	QuotaCents   int64
	DueDate      time.Time
}

// IssuancePreview is the preview of issuing one period (FR-9). When an active
// issuance already exists, Existing is set and Rows are not computed.
type IssuancePreview struct {
	BudgetID     string
	Year         int
	Periodicity  domain.Periodicity
	PeriodIndex  int
	Label        string
	DueDate      time.Time
	TotalCents   int64
	BaseCents    int64
	ReserveCents int64
	SumPPM       int
	// Rows has only the units that get a receivable (positive quota).
	Rows     []PreviewRow
	Existing *IssuanceSummary
}

// Viewer identifies who is asking for a statement, receipt or card.
type Viewer struct {
	UserID   string
	IsSyndic bool
}

// StatementRow is a receivable of a unit.
type StatementRow struct {
	Account domain.FinancialAccount
	// IsQuota is true for a quota; PeriodLabel is then the machine period label
	// and Period the metadata the views localise.
	IsQuota     bool
	PeriodLabel string
	Period      domain.QuotaPeriod
}

// Statement is the Conta corrente of one unit (FR-18..20).
type Statement struct {
	Unit model.Unit
	// Units are the choices of the unit selector (the viewer's active units, or all units for the syndic).
	Units   []model.Unit
	Pending []StatementRow // pending, including overdue, by due date
	Settled []StatementRow // settled, visible to the viewer (FR-19)
	// Canceled is filled for the syndic only.
	Canceled          []StatementRow
	PendingTotalCents int64
	OverdueCount      int
	OverdueTotalCents int64
}

// ReceiptRow is the store projection behind a Recibo.
type ReceiptRow struct {
	AccountID       string
	CondominiumName string
	Currency        money.Currency
	UnitID          string // empty when the receivable has no unit
	UnitCode        string
	Title           string
	AmountCents     int64
	Status          domain.AccountStatus
	SettlementDate  *time.Time
	IsQuota         bool
	PeriodLabel     string
	Period          domain.QuotaPeriod
	BaseCents       int64
	ReserveCents    int64
}

// ReceiptData is what the Recibo page renders (FR-26).
type ReceiptData struct {
	ReceiptRow
	// Number is the first 8 hex chars of the account id, upper-cased.
	Number string
}

// CardRows is the store projection behind the dashboard card.
type CardRows struct {
	// NextPending is the earliest pending quota of the units due today or later.
	NextPending *CardCharge
	// Overdue* count pending quotas of the units due before today.
	OverdueCount      int
	OverdueTotalCents int64
	// Budget is the approved budget of today's year, if any.
	Budget *Budget
}

// CardCharge is a quota amount and due date.
type CardCharge struct {
	AmountCents int64
	DueDate     time.Time
}

// ResidentCard is the dashboard card of a morador (FR-22).
type ResidentCard struct {
	NextPending       *CardCharge
	OverdueCount      int
	OverdueTotalCents int64
}

// SyndicCard is the dashboard card of the syndic (FR-22).
type SyndicCard struct {
	// HasBudget is false when the current year has no approved budget.
	HasBudget   bool
	BudgetID    string
	PeriodIndex int
	PeriodLabel string
	// Period is the current period (year, periodicity, index) to localise.
	Period domain.QuotaPeriod
	// Issued is true when the current period has an active issuance.
	Issued       bool
	IssuanceID   string
	ChargeCount  int
	SettledCount int
}

// CardData is the quotas card; either part is nil when it does not apply.
type CardData struct {
	Resident *ResidentCard
	Syndic   *SyndicCard
}

// QuotaService orchestrates budgets, issuances, statements, receipts and the card.
type QuotaService struct {
	Store       QuotaStore
	Occupancies OccupancyLister
	Audit       AuditRecorder
	// Currency resolves the condominium currency (EUR/BRL reserve default).
	Currency func(ctx context.Context, condoID string) (money.Currency, error)
	Now      func() time.Time
}

func (s *QuotaService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *QuotaService) audit(ctx context.Context, actorID string, ev model.AuditEventType, details map[string]any) error {
	if s.Audit == nil {
		return nil
	}
	var uid *string
	if actorID != "" {
		uid = &actorID
	}
	return s.Audit.RecordEvent(ctx, uid, ev, details)
}

func validParams(p BudgetParams) error {
	if !p.Periodicity.Valid() || p.DueDay < 1 || p.DueDay > 28 || p.ReserveRateBP < 0 || p.ReserveRateBP > 10000 {
		return ErrInvalidInput
	}
	return nil
}

func validLine(in LineInput) (LineInput, error) {
	in.Description = strings.TrimSpace(in.Description)
	if domain.ValidateTitle(in.Description) != nil || in.AmountCents <= 0 ||
		!in.Category.ValidFor(domain.AccountTypePayable) {
		return in, ErrInvalidInput
	}
	return in, nil
}

// DefaultReserveBP is the default reserve fund rate: 10% for EUR, 0% otherwise.
func DefaultReserveBP(c money.Currency) int {
	if c == money.EUR {
		return 1000
	}
	return 0
}

// CreateBudget creates a draft budget for a year (FR-3).
func (s *QuotaService) CreateBudget(ctx context.Context, actorID, condoID string, in BudgetInput) (Budget, error) {
	rate := 0
	if in.ReserveRateBP != nil {
		rate = *in.ReserveRateBP
	} else {
		cur, err := s.Currency(ctx, condoID)
		if err != nil {
			return Budget{}, err
		}
		rate = DefaultReserveBP(cur)
	}
	p := BudgetParams{Periodicity: in.Periodicity, DueDay: in.DueDay, ReserveRateBP: rate}
	if err := validParams(p); err != nil || in.Year < 2000 || in.Year > 2100 {
		return Budget{}, ErrInvalidInput
	}
	b := Budget{
		CondominiumID: condoID, Year: in.Year, Status: domain.BudgetDraft,
		Periodicity: p.Periodicity, DueDay: p.DueDay, ReserveRateBP: p.ReserveRateBP,
		CreatedBy: actorID,
	}
	id, err := s.Store.CreateBudget(ctx, b)
	if err != nil {
		return Budget{}, err
	}
	b.ID = id
	return b, s.audit(ctx, actorID, model.AuditBudgetCreated,
		map[string]any{"budget_id": id, "condominium_id": condoID, "year": b.Year})
}

// editBudget runs fn on the locked budget after re-checking it is a draft.
func (s *QuotaService) editBudget(ctx context.Context, actorID, condoID, budgetID string, fn func(b LockedBudget) error) error {
	var year int
	err := s.Store.WithLockedBudget(ctx, condoID, budgetID, func(_ context.Context, b LockedBudget) error {
		if !b.Budget().Status.CanEdit() {
			return domain.ErrBudgetLocked
		}
		year = b.Budget().Year
		return fn(b)
	})
	if err != nil {
		return err
	}
	return s.audit(ctx, actorID, model.AuditBudgetUpdated,
		map[string]any{"budget_id": budgetID, "condominium_id": condoID, "year": year})
}

// UpdateBudget changes the parameters of a draft budget.
func (s *QuotaService) UpdateBudget(ctx context.Context, actorID, condoID, budgetID string, p BudgetParams) error {
	if err := validParams(p); err != nil {
		return err
	}
	return s.editBudget(ctx, actorID, condoID, budgetID, func(b LockedBudget) error { return b.UpdateParams(p) })
}

// AddLine adds a line to a draft budget and returns its id (FR-4).
func (s *QuotaService) AddLine(ctx context.Context, actorID, condoID, budgetID string, in LineInput) (string, error) {
	in, err := validLine(in)
	if err != nil {
		return "", err
	}
	var id string
	err = s.editBudget(ctx, actorID, condoID, budgetID, func(b LockedBudget) (e error) {
		id, e = b.AddLine(BudgetLine{BudgetID: budgetID, Description: in.Description, Category: in.Category, AmountCents: in.AmountCents})
		return e
	})
	return id, err
}

// UpdateLine edits a line of a draft budget.
func (s *QuotaService) UpdateLine(ctx context.Context, actorID, condoID, budgetID, lineID string, in LineInput) error {
	in, err := validLine(in)
	if err != nil {
		return err
	}
	return s.editBudget(ctx, actorID, condoID, budgetID, func(b LockedBudget) error {
		return b.UpdateLine(BudgetLine{ID: lineID, BudgetID: budgetID, Description: in.Description, Category: in.Category, AmountCents: in.AmountCents})
	})
}

// RemoveLine removes a line of a draft budget.
func (s *QuotaService) RemoveLine(ctx context.Context, actorID, condoID, budgetID, lineID string) error {
	return s.editBudget(ctx, actorID, condoID, budgetID, func(b LockedBudget) error { return b.RemoveLine(lineID) })
}

// ApproveBudget approves a draft budget with at least one line (FR-5).
func (s *QuotaService) ApproveBudget(ctx context.Context, actorID, condoID, budgetID string) error {
	var year int
	err := s.Store.WithLockedBudget(ctx, condoID, budgetID, func(_ context.Context, b LockedBudget) error {
		if !b.Budget().Status.CanApprove() {
			return domain.ErrBudgetLocked
		}
		if len(b.Lines()) == 0 {
			return domain.ErrBudgetEmpty
		}
		year = b.Budget().Year
		return b.SetStatus(domain.BudgetApproved)
	})
	if err != nil {
		return err
	}
	return s.audit(ctx, actorID, model.AuditBudgetApproved,
		map[string]any{"budget_id": budgetID, "condominium_id": condoID, "year": year})
}

// ReopenBudget returns an approved budget to draft when it has no active issuance (FR-6).
func (s *QuotaService) ReopenBudget(ctx context.Context, actorID, condoID, budgetID string) error {
	var year int
	err := s.Store.WithLockedBudget(ctx, condoID, budgetID, func(_ context.Context, b LockedBudget) error {
		if !b.Budget().Status.CanReopen() {
			return domain.ErrBudgetNotApproved
		}
		has, err := b.HasActiveIssuance()
		if err != nil {
			return err
		}
		if has {
			return domain.ErrBudgetHasIssuances
		}
		year = b.Budget().Year
		return b.SetStatus(domain.BudgetDraft)
	})
	if err != nil {
		return err
	}
	return s.audit(ctx, actorID, model.AuditBudgetReopened,
		map[string]any{"budget_id": budgetID, "condominium_id": condoID, "year": year})
}

// ListBudgets lists the condominium's budgets.
func (s *QuotaService) ListBudgets(ctx context.Context, condoID string) ([]BudgetSummary, error) {
	return s.Store.ListBudgets(ctx, condoID)
}

func sumLines(lines []BudgetLine) int64 {
	var t int64
	for _, l := range lines {
		t += l.AmountCents
	}
	return t
}

func sumShares(shares []domain.ShareInput) int {
	t := 0
	for _, sh := range shares {
		if sh.SharePPM > 0 {
			t += sh.SharePPM
		}
	}
	return t
}

// periodAmounts returns base and reserve of a period (1-based) of a budget.
func periodAmounts(b Budget, total int64, periodIndex int) (base, reserve int64) {
	parts := domain.SplitAcrossPeriods(total, b.Periodicity.Periods())
	base = parts[periodIndex-1]
	return base, domain.ReserveFor(base, b.ReserveRateBP)
}

// GetBudget returns a budget with its total and the per-period, per-unit quota preview (FR-4).
func (s *QuotaService) GetBudget(ctx context.Context, condoID, id string) (BudgetDetail, error) {
	d, err := s.Store.GetBudget(ctx, condoID, id)
	if err != nil {
		return BudgetDetail{}, err
	}
	d.TotalCents = sumLines(d.Lines)
	shares, err := s.Store.UnitShares(ctx, condoID)
	if err != nil {
		return BudgetDetail{}, err
	}
	d.SumPPM = sumShares(shares)
	n := d.Periodicity.Periods()
	for _, sh := range shares {
		if sh.SharePPM > 0 {
			d.Units = append(d.Units, UnitQuotas{UnitID: sh.UnitID, Code: sh.Code, SharePPM: sh.SharePPM, PerPeriodCents: make([]int64, n)})
		}
	}
	for i := 1; i <= n && d.TotalCents > 0; i++ {
		base, reserve := periodAmounts(d.Budget, d.TotalCents, i)
		d.Periods = append(d.Periods, PeriodQuota{
			Index: i, Label: domain.PeriodLabel(d.Year, d.Periodicity, i),
			DueDate:   domain.DueDate(d.Year, d.Periodicity, i, d.DueDay),
			BaseCents: base, ReserveCents: reserve, TotalCents: base + reserve,
		})
		for u, a := range chargeable(domain.AllocateByShare(base, reserve, shares)) {
			d.Units[u].PerPeriodCents[i-1] = a.TotalCents()
		}
	}
	return d, nil
}

// chargeable returns the allocations of units with a positive share (zero
// quotas included, so the index matches UnitQuotas).
func chargeable(all []domain.Allocation) []domain.Allocation {
	var out []domain.Allocation
	for _, a := range all {
		if a.SharePPM > 0 {
			out = append(out, a)
		}
	}
	return out
}

// computeIssuance applies the FR-12 refusals and the allocation (pure).
func computeIssuance(b Budget, lines []BudgetLine, shares []domain.ShareInput, periodIndex int) (IssuancePreview, error) {
	p := IssuancePreview{BudgetID: b.ID, Year: b.Year, Periodicity: b.Periodicity, PeriodIndex: periodIndex, SumPPM: sumShares(shares)}
	if b.Status != domain.BudgetApproved {
		return p, domain.ErrBudgetNotApproved
	}
	if periodIndex < 1 || periodIndex > b.Periodicity.Periods() {
		return p, domain.ErrPeriodOutOfRange
	}
	p.Label = domain.PeriodLabel(b.Year, b.Periodicity, periodIndex)
	p.DueDate = domain.DueDate(b.Year, b.Periodicity, periodIndex, b.DueDay)
	if p.SumPPM != permille.Total {
		return p, SharesError{SumPPM: p.SumPPM}
	}
	total := sumLines(lines)
	if total <= 0 {
		return p, domain.ErrBudgetEmpty
	}
	p.BaseCents, p.ReserveCents = periodAmounts(b, total, periodIndex)
	p.TotalCents = p.BaseCents + p.ReserveCents
	for _, a := range domain.AllocateByShare(p.BaseCents, p.ReserveCents, shares) {
		if a.SharePPM <= 0 || a.TotalCents() <= 0 {
			continue // no receivable for zero shares or a zero quota
		}
		p.Rows = append(p.Rows, PreviewRow{
			UnitID: a.UnitID, Code: a.Code, SharePPM: a.SharePPM,
			BaseCents: a.BaseCents, ReserveCents: a.ReserveCents,
			QuotaCents: a.TotalCents(), DueDate: p.DueDate,
		})
	}
	if len(p.Rows) == 0 {
		return p, domain.ErrNoChargeableUnits
	}
	return p, nil
}

// PreviewIssuance previews issuing a period (FR-9). When the period already has
// an active issuance it returns a preview with Existing set and no error.
func (s *QuotaService) PreviewIssuance(ctx context.Context, condoID, budgetID string, periodIndex int) (IssuancePreview, error) {
	if ex, ok, err := s.Store.ActiveIssuanceFor(ctx, condoID, budgetID, periodIndex); err != nil {
		return IssuancePreview{}, err
	} else if ok {
		return IssuancePreview{BudgetID: budgetID, Year: ex.Year, Periodicity: ex.Periodicity, PeriodIndex: periodIndex, Label: ex.Label(),
			TotalCents: ex.TotalCents, BaseCents: ex.BaseCents, ReserveCents: ex.ReserveCents, Existing: &ex}, nil
	}
	d, err := s.Store.GetBudget(ctx, condoID, budgetID)
	if err != nil {
		return IssuancePreview{}, err
	}
	shares, err := s.Store.UnitShares(ctx, condoID)
	if err != nil {
		return IssuancePreview{}, err
	}
	return computeIssuance(d.Budget, d.Lines, shares, periodIndex)
}

// Issue creates the issuance of a period: one pending receivable per unit with
// a positive quota, atomically (FR-10..12). Re-issuing an active period creates
// nothing and returns the existing issuance id with created=false (FR-11).
func (s *QuotaService) Issue(ctx context.Context, actorID, condoID, budgetID string, periodIndex int) (string, bool, error) {
	// An existing active issuance wins over any later refusal (for example the
	// shares changed since, FR-17). A concurrent issuer is caught by the
	// database unique index inside the transaction.
	if ex, ok, err := s.Store.ActiveIssuanceFor(ctx, condoID, budgetID, periodIndex); err != nil {
		return "", false, err
	} else if ok {
		return ex.ID, false, nil
	}
	var (
		id      string
		created bool
		pv      IssuancePreview
	)
	err := s.Store.WithLockedIssue(ctx, condoID, budgetID, func(_ context.Context, b LockedIssue) error {
		shares, err := b.Shares()
		if err != nil {
			return err
		}
		bud := b.Budget() // status re-checked here, inside the lock
		pv, err = computeIssuance(bud, b.Lines(), shares, periodIndex)
		if err != nil {
			return err
		}
		id, created, err = b.InsertIssuance(NewIssuance{
			BudgetID: budgetID, PeriodIndex: periodIndex, TotalCents: pv.TotalCents,
			BaseCents: pv.BaseCents, ReserveCents: pv.ReserveCents, CreatedBy: actorID,
		})
		if err != nil || !created {
			return err
		}
		charges := make([]NewCharge, len(pv.Rows))
		for i, r := range pv.Rows {
			charges[i] = NewCharge{
				UnitID: r.UnitID, SharePPM: r.SharePPM, Title: "Quota " + pv.Label + " " + r.Code, // neutral fallback; views render the localised title from the quota period
				AmountCents: r.QuotaCents, BaseCents: r.BaseCents, ReserveCents: r.ReserveCents,
				DueDate: r.DueDate, CreatedBy: actorID,
			}
		}
		return b.InsertCharges(id, charges)
	})
	if err != nil {
		return "", false, err
	}
	if !created {
		return id, false, nil
	}
	err = s.audit(ctx, actorID, model.AuditQuotaIssued, map[string]any{
		"issuance_id": id, "budget_id": budgetID, "condominium_id": condoID,
		"period": pv.Label, "total_cents": pv.TotalCents, "charges": len(pv.Rows),
	})
	return id, true, err
}

// CancelIssuance cancels a whole issuance when none of its quotas is settled
// (FR-15). It locks only the issuance row.
func (s *QuotaService) CancelIssuance(ctx context.Context, actorID, condoID, issuanceID string) error {
	var iss IssuanceSummary
	err := s.Store.WithLockedIssuance(ctx, condoID, issuanceID, func(_ context.Context, i LockedIssuance) error {
		iss = i.Issuance()
		if !iss.Status.CanCancel() {
			return ErrIssuanceNotActive
		}
		affected, total, err := i.CancelPendingCharges()
		if err != nil {
			return err
		}
		if affected != total {
			return domain.ErrIssuanceHasSettled // rolls the transaction back
		}
		return i.MarkCanceled(s.now())
	})
	if err != nil {
		return err
	}
	return s.audit(ctx, actorID, model.AuditQuotaIssuanceCanceled, map[string]any{
		"issuance_id": issuanceID, "budget_id": iss.BudgetID, "condominium_id": condoID, "period": iss.Label(),
	})
}

// ListIssuances lists the issuances of a budget (FR-14).
func (s *QuotaService) ListIssuances(ctx context.Context, condoID, budgetID string) ([]IssuanceSummary, error) {
	return s.Store.ListIssuances(ctx, condoID, budgetID)
}

func utcDate(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// canSeeSettled applies FR-19: an occupant sees settled receivables whose
// settlement date is on or after the (UTC) date the occupancy started.
func canSeeSettled(settlement *time.Time, startedAt time.Time) bool {
	return settlement != nil && !utcDate(*settlement).Before(utcDate(startedAt))
}

func (s *QuotaService) denied(ctx context.Context, viewer Viewer, details map[string]any) {
	uid := viewer.UserID
	_ = s.audit(ctx, uid, model.AuditAccessDenied, details)
}

// Statement returns the Conta corrente of a unit (FR-18..20). An empty unitID
// selects the first unit by code. The syndic sees every unit and everything;
// an occupant sees only their active units. Other requests get ErrForbidden and
// are audited; a user without any unit gets ErrNoUnit.
func (s *QuotaService) Statement(ctx context.Context, viewer Viewer, condoID, unitID string) (Statement, error) {
	var st Statement
	var startedAt time.Time
	if viewer.IsSyndic {
		shares, err := s.Store.UnitShares(ctx, condoID)
		if err != nil {
			return st, err
		}
		for _, sh := range shares {
			u := model.Unit{ID: sh.UnitID, CondominiumID: condoID, Code: sh.Code, SharePPM: sh.SharePPM}
			st.Units = append(st.Units, u)
			if unitID == "" && st.Unit.ID == "" || unitID != "" && sh.UnitID == unitID {
				st.Unit = u
			}
		}
		if st.Unit.ID == "" {
			if unitID == "" {
				return st, ErrNoUnit
			}
			return st, ErrNotFound
		}
	} else {
		occ, err := s.Occupancies.ListActiveOccupanciesForUser(ctx, viewer.UserID, condoID)
		if err != nil {
			return st, err
		}
		if len(occ) == 0 {
			return st, ErrNoUnit
		}
		for _, o := range occ {
			st.Units = append(st.Units, o.Unit)
		}
		chosen := occ[0]
		if unitID != "" {
			found := false
			for _, o := range occ {
				if o.Unit.ID == unitID {
					chosen, found = o, true
					break
				}
			}
			if !found {
				s.denied(ctx, viewer, map[string]any{"resource": "statement", "unit_id": unitID, "condominium_id": condoID})
				return st, ErrForbidden
			}
		}
		st.Unit, startedAt = chosen.Unit, chosen.StartedAt
	}

	rows, err := s.Store.UnitReceivables(ctx, condoID, st.Unit.ID)
	if err != nil {
		return st, err
	}
	today := utcDate(s.now())
	for _, r := range rows {
		a := r.Account
		switch a.Status {
		case domain.StatusPending:
			st.Pending = append(st.Pending, r)
			st.PendingTotalCents += a.AmountCents
			if a.DueDate.Before(today) {
				st.OverdueCount++
				st.OverdueTotalCents += a.AmountCents
			}
		case domain.StatusSettled:
			if viewer.IsSyndic || canSeeSettled(a.SettlementDate, startedAt) {
				st.Settled = append(st.Settled, r)
			}
		case domain.StatusCanceled:
			if viewer.IsSyndic {
				st.Canceled = append(st.Canceled, r)
			}
		}
	}
	return st, nil
}

// Receipt returns the Recibo of a settled receivable (FR-21, FR-26): the
// syndic for any settled receivable of the condominium, an occupant when FR-19
// lets them see it. Unauthorized requests get ErrForbidden and are audited; a
// missing or non-settled receivable is ErrNotFound.
func (s *QuotaService) Receipt(ctx context.Context, viewer Viewer, condoID, accountID string) (ReceiptData, error) {
	row, err := s.Store.ReceiptData(ctx, condoID, accountID)
	if err != nil {
		return ReceiptData{}, err
	}
	deny := func() (ReceiptData, error) {
		s.denied(ctx, viewer, map[string]any{"resource": "receipt", "account_id": accountID, "unit_id": row.UnitID, "condominium_id": condoID})
		return ReceiptData{}, ErrForbidden
	}
	if !viewer.IsSyndic {
		if row.UnitID == "" {
			return deny()
		}
		occ, err := s.Occupancies.ListActiveOccupanciesForUser(ctx, viewer.UserID, condoID)
		if err != nil {
			return ReceiptData{}, err
		}
		var started *time.Time
		for _, o := range occ {
			if o.Unit.ID == row.UnitID {
				t := o.StartedAt
				started = &t
			}
		}
		if started == nil {
			return deny()
		}
		if row.Status != domain.StatusSettled {
			return ReceiptData{}, ErrNotFound
		}
		if !canSeeSettled(row.SettlementDate, *started) {
			return deny()
		}
	}
	if row.Status != domain.StatusSettled || row.SettlementDate == nil {
		return ReceiptData{}, ErrNotFound
	}
	num := strings.ToUpper(strings.ReplaceAll(row.AccountID, "-", ""))
	if len(num) > 8 {
		num = num[:8]
	}
	return ReceiptData{ReceiptRow: row, Number: num}, nil
}

// periodIndexOf returns the period (1-based) of the budget containing day.
func periodIndexOf(p domain.Periodicity, day time.Time) int {
	if p == domain.PeriodicityQuarterly {
		return (int(day.Month())-1)/3 + 1
	}
	return int(day.Month())
}

// Card returns the dashboard quotas card (FR-22): the resident part when the
// viewer occupies units, the syndic part for the syndic.
func (s *QuotaService) Card(ctx context.Context, viewer Viewer, condoID string, today time.Time) (CardData, error) {
	today = utcDate(today)
	var out CardData
	var unitIDs []string
	if s.Occupancies != nil && viewer.UserID != "" {
		occ, err := s.Occupancies.ListActiveOccupanciesForUser(ctx, viewer.UserID, condoID)
		if err != nil {
			return out, err
		}
		for _, o := range occ {
			unitIDs = append(unitIDs, o.Unit.ID)
		}
	}
	if len(unitIDs) == 0 && !viewer.IsSyndic {
		return out, nil
	}
	rows, err := s.Store.CardData(ctx, condoID, unitIDs, today)
	if err != nil {
		return out, err
	}
	if len(unitIDs) > 0 {
		out.Resident = &ResidentCard{NextPending: rows.NextPending, OverdueCount: rows.OverdueCount, OverdueTotalCents: rows.OverdueTotalCents}
	}
	if viewer.IsSyndic {
		sc := &SyndicCard{}
		if rows.Budget != nil && rows.Budget.Status == domain.BudgetApproved {
			b := rows.Budget
			sc.HasBudget, sc.BudgetID = true, b.ID
			sc.PeriodIndex = periodIndexOf(b.Periodicity, today)
			sc.PeriodLabel = domain.PeriodLabel(b.Year, b.Periodicity, sc.PeriodIndex)
			sc.Period = domain.QuotaPeriod{Year: b.Year, Periodicity: b.Periodicity, Index: sc.PeriodIndex}
			iss, ok, err := s.Store.ActiveIssuanceFor(ctx, condoID, b.ID, sc.PeriodIndex)
			if err != nil {
				return out, err
			}
			if ok {
				sc.Issued, sc.IssuanceID = true, iss.ID
				sc.ChargeCount, sc.SettledCount = iss.ChargeCount, iss.SettledCount
			}
		}
		out.Syndic = sc
	}
	return out, nil
}
