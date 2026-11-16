package domain

import (
	"errors"
	"math/big"
	"time"
)

// Late fee and extraordinary quota domain errors.
var (
	// ErrInvalidInstallments means the installment count is outside the allowed range.
	ErrInvalidInstallments = errors.New("invalid number of installments")
	// ErrInvalidExtraordinary means the extraordinary quota input is invalid.
	ErrInvalidExtraordinary = errors.New("invalid extraordinary quota")
	// ErrExtraordinaryHasSettled means some installment is already settled.
	ErrExtraordinaryHasSettled = errors.New("extraordinary quota has settled installments")
)

// QuotaKind tells an ordinary (budget) quota from an extraordinary one.
// The empty value means ordinary.
type QuotaKind string

// Quota kinds.
const (
	QuotaKindOrdinary      QuotaKind = "ordinary"
	QuotaKindExtraordinary QuotaKind = "extraordinary"
)

// LateFeeRates are the late fee parameters: fine and monthly interest in basis
// points and the grace period in days.
type LateFeeRates struct {
	FeeBP      int
	InterestBP int
	GraceDays  int
}

// LateFee is the computed late fee of a quota settled after its due date.
type LateFee struct {
	DaysLate      int
	FineCents     int64
	InterestCents int64
	TotalCents    int64
}

// LateFeeDetail is the persisted breakdown of an applied late fee, with the
// rates that were in force when it was applied.
type LateFeeDetail struct {
	FineCents     int64
	InterestCents int64
	DaysLate      int
	FeeBP         int
	InterestBP    int
	GraceDays     int
}

// LateFeeOrigin links a late fee receivable to the quota it originates from.
type LateFeeOrigin struct {
	Origin QuotaPeriod
	Detail LateFeeDetail
}

// truncateDate returns t as a UTC calendar date at midnight.
func truncateDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ComputeLateFee returns the late fee of a quota of amountCents due on due and
// settled on settledOn. Days late = settledOn - (due + grace) counted on UTC
// calendar dates; when not positive there is no fee. Fine = amount*FeeBP/10000
// and interest = amount*InterestBP*days/(10000*30), each rounded half up to the
// cent; total is their sum. Integer arithmetic only (big.Int, so no overflow).
func ComputeLateFee(amountCents int64, due, settledOn time.Time, r LateFeeRates) LateFee {
	if amountCents <= 0 {
		return LateFee{}
	}
	grace := r.GraceDays
	if grace < 0 {
		grace = 0
	}
	limit := truncateDate(due).AddDate(0, 0, grace)
	days := int(truncateDate(settledOn).Sub(limit) / (24 * time.Hour))
	if days <= 0 {
		return LateFee{}
	}
	fine := divRoundHalfUp(amountCents, int64(max(r.FeeBP, 0)), 1, 10000)
	interest := divRoundHalfUp(amountCents, int64(max(r.InterestBP, 0)), int64(days), 10000*30)
	return LateFee{DaysLate: days, FineCents: fine, InterestCents: interest, TotalCents: fine + interest}
}

// divRoundHalfUp returns round_half_up(a*b*c/den) for non-negative a, b, c.
func divRoundHalfUp(a, b, c, den int64) int64 {
	n := new(big.Int).Mul(big.NewInt(a), big.NewInt(b))
	n.Mul(n, big.NewInt(c))
	n.Mul(n, big.NewInt(2))
	n.Add(n, big.NewInt(den))
	n.Quo(n, big.NewInt(2*den))
	return n.Int64()
}

// AllocateEqual splits totalCents equally among the units with SharePPM > 0,
// by largest remainder (the extra cents go to the lowest Code, then UnitID).
// The result has one entry per input, in input order, with the amount in
// BaseCents; units with SharePPM <= 0 get zero. With no positive share, or a
// non-positive total, all amounts are zero.
func AllocateEqual(totalCents int64, shares []ShareInput) []Allocation {
	out := make([]Allocation, len(shares))
	eq := make([]ShareInput, len(shares))
	var n uint64
	for i, s := range shares {
		out[i] = Allocation{UnitID: s.UnitID, Code: s.Code, SharePPM: s.SharePPM}
		eq[i] = ShareInput{UnitID: s.UnitID, Code: s.Code}
		if s.SharePPM > 0 {
			eq[i].SharePPM = 1
			n++
		}
	}
	if n == 0 {
		return out
	}
	for i, v := range allocate(totalCents, n, eq) {
		out[i].BaseCents = v
	}
	return out
}

// SplitInstallments splits totalCents across n installments (earlier ones get
// the extra cents). It returns nil when n <= 0 or total < 0.
func SplitInstallments(totalCents int64, n int) []int64 {
	return SplitAcrossPeriods(totalCents, n)
}

// InstallmentDueDate returns the due date of installment n (1..N): first moved
// by n-1 months, on the same day. first's day must be 1..28.
func InstallmentDueDate(first time.Time, n int) time.Time {
	f := truncateDate(first)
	return time.Date(f.Year(), f.Month()+time.Month(n-1), f.Day(), 0, 0, 0, 0, time.UTC)
}
