package domain

import (
	"errors"
	"fmt"
	"math/bits"
	"sort"
	"time"
)

// Quota domain errors.
var (
	// ErrSharesNot100 means the unit shares do not add up to exactly 100%.
	ErrSharesNot100 = errors.New("unit shares do not sum to 100%")
	// ErrBudgetNotApproved means the budget is not approved, so it cannot be issued.
	ErrBudgetNotApproved = errors.New("budget is not approved")
	// ErrPeriodOutOfRange means the period does not belong to the budget's year.
	ErrPeriodOutOfRange = errors.New("period out of range for the budget")
	// ErrNoChargeableUnits means no unit has a share greater than zero.
	ErrNoChargeableUnits = errors.New("no unit has a share greater than zero")
	// ErrBudgetLocked means the budget is approved and cannot be edited.
	ErrBudgetLocked = errors.New("budget is approved and cannot be edited")
	// ErrBudgetHasIssuances means the budget has active issuances.
	ErrBudgetHasIssuances = errors.New("budget has active issuances")
	// ErrIssuanceHasSettled means some quota of the issuance is already settled.
	ErrIssuanceHasSettled = errors.New("issuance has settled quotas")
	// ErrBudgetEmpty means the budget has no lines (total is zero).
	ErrBudgetEmpty = errors.New("budget has no lines")
	// ErrBudgetExists means a budget already exists for the condominium and year.
	ErrBudgetExists = errors.New("budget already exists for this year")
)

// Periodicity is how often a budget is issued.
type Periodicity string

// Supported periodicities.
const (
	PeriodicityMonthly   Periodicity = "monthly"
	PeriodicityQuarterly Periodicity = "quarterly"
)

// Valid reports whether p is a known periodicity.
func (p Periodicity) Valid() bool {
	return p == PeriodicityMonthly || p == PeriodicityQuarterly
}

// Periods returns the number of periods in a year (12 or 4); 0 for an unknown value.
func (p Periodicity) Periods() int {
	switch p {
	case PeriodicityMonthly:
		return 12
	case PeriodicityQuarterly:
		return 4
	}
	return 0
}

// BudgetStatus is the lifecycle state of a budget.
type BudgetStatus string

// Budget states.
const (
	BudgetDraft    BudgetStatus = "draft"
	BudgetApproved BudgetStatus = "approved"
)

// CanEdit reports whether the budget parameters and lines may change.
func (s BudgetStatus) CanEdit() bool { return s == BudgetDraft }

// CanApprove reports whether the budget may be approved.
func (s BudgetStatus) CanApprove() bool { return s == BudgetDraft }

// CanReopen reports whether the budget may go back to draft (also needs no active issuance).
func (s BudgetStatus) CanReopen() bool { return s == BudgetApproved }

// IssuanceStatus is the state of a quota issuance.
type IssuanceStatus string

// Issuance states.
const (
	IssuanceIssued   IssuanceStatus = "issued"
	IssuanceCanceled IssuanceStatus = "canceled"
)

// CanCancel reports whether the issuance may be canceled (settled-quota check is separate).
func (s IssuanceStatus) CanCancel() bool { return s == IssuanceIssued }

// SplitAcrossPeriods splits totalCents across n periods by largest remainder
// over equal weights: the first total%n periods get one extra cent, so the
// parts always sum to totalCents. It returns nil when n <= 0 or total < 0.
func SplitAcrossPeriods(totalCents int64, periods int) []int64 {
	if periods <= 0 || totalCents < 0 {
		return nil
	}
	n := int64(periods)
	base, extra := totalCents/n, totalCents%n
	out := make([]int64, periods)
	for i := range out {
		out[i] = base
		if int64(i) < extra {
			out[i]++
		}
	}
	return out
}

// ReserveFor returns base * rateBP / 10000 (rateBP in basis points) rounded
// half up to the cent, using integer arithmetic only. Non-positive inputs give 0.
func ReserveFor(baseCents int64, rateBP int) int64 {
	if baseCents <= 0 || rateBP <= 0 {
		return 0
	}
	hi, lo := bits.Mul64(uint64(baseCents), uint64(rateBP))
	lo, carry := bits.Add64(lo, 5000, 0)
	hi += carry
	q, _ := bits.Div64(hi, lo, 10000) // hi < 10000 for any realistic input
	return int64(q)
}

// ShareInput is a unit and its frozen share (ppm) for allocation.
type ShareInput struct {
	UnitID   string
	Code     string
	SharePPM int
}

// Allocation is the quota of one unit: base and reserve parts; the quota is their sum.
type Allocation struct {
	UnitID       string
	Code         string
	SharePPM     int
	BaseCents    int64
	ReserveCents int64
}

// TotalCents is the unit's quota (base plus reserve).
func (a Allocation) TotalCents() int64 { return a.BaseCents + a.ReserveCents }

// AllocateByShare splits baseCents and reserveCents separately across the
// units by SharePPM, using largest remainder over the sum of the positive
// shares; ties go to the lowest Code (lexicographic, then UnitID). The result
// has one entry per input, in input order; units with SharePPM <= 0 are
// returned with zero amounts (callers create no receivable for them). The sum
// of each part always equals its total. With no positive share it returns
// zero allocations. Products are computed with 128-bit arithmetic, so there is
// no overflow bound.
func AllocateByShare(baseCents, reserveCents int64, shares []ShareInput) []Allocation {
	out := make([]Allocation, len(shares))
	var sum uint64
	for i, s := range shares {
		out[i] = Allocation{UnitID: s.UnitID, Code: s.Code, SharePPM: s.SharePPM}
		if s.SharePPM > 0 {
			sum += uint64(s.SharePPM)
		}
	}
	if sum == 0 {
		return out
	}
	base := allocate(baseCents, sum, shares)
	reserve := allocate(reserveCents, sum, shares)
	for i := range out {
		out[i].BaseCents = base[i]
		out[i].ReserveCents = reserve[i]
	}
	return out
}

// allocate distributes total over the positive shares (denominator sum).
func allocate(total int64, sum uint64, shares []ShareInput) []int64 {
	out := make([]int64, len(shares))
	if total <= 0 {
		return out
	}
	type rem struct {
		idx int
		r   uint64
	}
	var rems []rem
	var given int64
	for i, s := range shares {
		if s.SharePPM <= 0 {
			continue
		}
		hi, lo := bits.Mul64(uint64(total), uint64(s.SharePPM))
		q, r := bits.Div64(hi, lo, sum) // hi < sum because ppm <= sum
		out[i] = int64(q)
		given += int64(q)
		rems = append(rems, rem{i, r})
	}
	sort.SliceStable(rems, func(a, b int) bool {
		if rems[a].r != rems[b].r {
			return rems[a].r > rems[b].r
		}
		sa, sb := shares[rems[a].idx], shares[rems[b].idx]
		if sa.Code != sb.Code {
			return sa.Code < sb.Code
		}
		return sa.UnitID < sb.UnitID
	})
	for k := int64(0); k < total-given && int(k) < len(rems); k++ {
		out[rems[k].idx]++
	}
	return out
}

// PeriodMonth returns the first calendar month (1..12) of the period.
func PeriodMonth(p Periodicity, periodIndex int) time.Month {
	if p == PeriodicityQuarterly {
		return time.Month(3*(periodIndex-1) + 1)
	}
	return time.Month(periodIndex)
}

// DueDate returns the UTC due date of a period: dueDay in the period's month
// (monthly) or in the first month of the quarter (quarterly). dueDay is 1..28.
func DueDate(year int, p Periodicity, periodIndex, dueDay int) time.Time {
	return time.Date(year, PeriodMonth(p, periodIndex), dueDay, 0, 0, 0, 0, time.UTC)
}

// QuotaPeriod is the display metadata of a quota receivable: the period it
// charges and the unit code. It is rendered per locale at display time; the
// stored account title is only a neutral fallback.
type QuotaPeriod struct {
	Year        int
	Periodicity Periodicity
	Index       int
	UnitCode    string

	// Extraordinary quotas only; zero values mean an ordinary quota.
	Kind         QuotaKind
	Description  string
	Installment  int
	Installments int
}

// PeriodLabel returns a language-neutral machine label ("2026-03" monthly or
// "2026-T2" quarterly), for keys and URLs; it is not shown to users.
func PeriodLabel(year int, p Periodicity, periodIndex int) string {
	if p == PeriodicityQuarterly {
		return fmt.Sprintf("%d-T%d", year, periodIndex)
	}
	return fmt.Sprintf("%d-%02d", year, periodIndex)
}
