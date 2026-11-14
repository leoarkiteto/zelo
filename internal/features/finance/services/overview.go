package services

import (
	"context"
	"sort"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
)

// MonthFlow is the realized flow of one month. Sparse: months without
// settlements are absent. Month is time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).
type MonthFlow struct {
	Month    time.Time
	InCents  int64
	OutCents int64
}

// CategoryTotal is the realized total of one (type, category) pair.
type CategoryTotal struct {
	Type     domain.AccountType
	Category domain.Category
	Cents    int64
}

// CurrentPosition is the "now" position, independent of the selected period.
type CurrentPosition struct {
	ReceivableCents int64
	PayableCents    int64
	OverdueCents    int64
}

// MonthPoint is one dense month of the overview charts.
type MonthPoint struct {
	Month        time.Time
	InCents      int64
	OutCents     int64
	BalanceCents int64
}

// CategoryShare is one category's share of a type total in the period.
type CategoryShare struct {
	Category domain.Category
	Cents    int64
	Percent  float64 // 0..100
}

// Overview is the presentation-ready financial overview of a condominium.
type Overview struct {
	Period                                    domain.Period
	Months                                    []MonthPoint
	TotalInCents, TotalOutCents, BalanceCents int64
	InByCategory, OutByCategory               []CategoryShare // desc, zero omitted
	Current                                   CurrentPosition // always "now", independent of Period
	Years                                     []int
	Empty                                     bool // TotalIn == 0 && TotalOut == 0
}

type monthKey struct {
	year  int
	month time.Month
}

// Overview builds the realized overview of the period plus the current
// position. today is derived once as s.Now().UTC().
func (s *FinanceService) Overview(ctx context.Context, condominiumID string, p domain.Period) (Overview, error) {
	today := s.Now().UTC()
	curStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	curEnd := curStart.AddDate(0, 1, 0)

	flows, err := s.Overviews.MonthlyRealized(ctx, condominiumID, p.Start, p.End)
	if err != nil {
		return Overview{}, err
	}
	cats, err := s.Overviews.RealizedByCategory(ctx, condominiumID, p.Start, p.End)
	if err != nil {
		return Overview{}, err
	}
	cur, err := s.Overviews.CurrentPosition(ctx, condominiumID, curStart, curEnd, today)
	if err != nil {
		return Overview{}, err
	}
	years, err := s.Overviews.SettlementYears(ctx, condominiumID)
	if err != nil {
		return Overview{}, err
	}

	byMonth := make(map[monthKey]MonthFlow, len(flows))
	for _, f := range flows {
		k := monthKey{f.Month.Year(), f.Month.Month()}
		agg := byMonth[k]
		agg.InCents += f.InCents
		agg.OutCents += f.OutCents
		byMonth[k] = agg
	}

	ov := Overview{Period: p, Current: cur, Years: years}
	for _, m := range p.Months() {
		f := byMonth[monthKey{m.Year(), m.Month()}]
		ov.Months = append(ov.Months, MonthPoint{
			Month:        m,
			InCents:      f.InCents,
			OutCents:     f.OutCents,
			BalanceCents: f.InCents - f.OutCents,
		})
		ov.TotalInCents += f.InCents
		ov.TotalOutCents += f.OutCents
	}
	ov.BalanceCents = ov.TotalInCents - ov.TotalOutCents
	ov.Empty = ov.TotalInCents == 0 && ov.TotalOutCents == 0

	ov.InByCategory = shares(cats, domain.AccountTypeReceivable)
	ov.OutByCategory = shares(cats, domain.AccountTypePayable)
	return ov, nil
}

// shares builds the sorted category shares of one type. Categories without a
// positive value are omitted; percentages are relative to the type total.
func shares(cats []CategoryTotal, t domain.AccountType) []CategoryShare {
	sum := map[domain.Category]int64{}
	for _, c := range cats {
		if c.Type == t && c.Cents > 0 {
			sum[c.Category] += c.Cents
		}
	}
	var total int64
	out := make([]CategoryShare, 0, len(sum))
	for cat, cents := range sum {
		total += cents
		out = append(out, CategoryShare{Category: cat, Cents: cents})
	}
	for i := range out {
		out[i].Percent = float64(out[i].Cents) * 100 / float64(total)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cents != out[j].Cents {
			return out[i].Cents > out[j].Cents
		}
		return out[i].Category < out[j].Category
	})
	return out
}
