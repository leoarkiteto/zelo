package services

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
)

type fakeOverviewStore struct {
	flows []MonthFlow
	cats  []CategoryTotal
	cur   CurrentPosition
	years []int
	err   error

	gotStart, gotEnd                time.Time
	gotMonthStart, gotMonthEnd, got time.Time
}

func (f *fakeOverviewStore) MonthlyRealized(_ context.Context, _ string, start, end time.Time) ([]MonthFlow, error) {
	f.gotStart, f.gotEnd = start, end
	return f.flows, f.err
}
func (f *fakeOverviewStore) RealizedByCategory(context.Context, string, time.Time, time.Time) ([]CategoryTotal, error) {
	return f.cats, f.err
}
func (f *fakeOverviewStore) CurrentPosition(_ context.Context, _ string, ms, me, today time.Time) (CurrentPosition, error) {
	f.gotMonthStart, f.gotMonthEnd, f.got = ms, me, today
	return f.cur, f.err
}
func (f *fakeOverviewStore) SettlementYears(context.Context, string) ([]int, error) {
	return f.years, f.err
}

func overviewSvc(st *fakeOverviewStore, now time.Time) *FinanceService {
	return &FinanceService{Overviews: st, Now: func() time.Time { return now }}
}

func TestOverviewDensifiesAndTotals(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	west := time.FixedZone("west", -3*3600)
	st := &fakeOverviewStore{
		// Non-UTC month keys: must still land in the right (year, month).
		flows: []MonthFlow{
			{Month: time.Date(2026, 11, 1, 0, 0, 0, 0, west), InCents: 1000, OutCents: 400},
			{Month: time.Date(2026, 9, 1, 0, 0, 0, 0, west), InCents: 200, OutCents: 500},
		},
		cur:   CurrentPosition{ReceivableCents: 1, PayableCents: 2, OverdueCents: 3},
		years: []int{2026, 2025},
	}
	p := domain.ParsePeriod("", now)
	ov, err := overviewSvc(st, now).Overview(context.Background(), "c1", p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Months) != 12 {
		t.Fatalf("months = %d", len(ov.Months))
	}
	byM := map[time.Month]MonthPoint{}
	for _, m := range ov.Months {
		byM[m.Month.Month()] = m
	}
	if m := byM[time.November]; m.InCents != 1000 || m.OutCents != 400 || m.BalanceCents != 600 {
		t.Fatalf("nov = %+v", m)
	}
	if m := byM[time.September]; m.InCents != 200 || m.OutCents != 500 || m.BalanceCents != -300 {
		t.Fatalf("sep = %+v", m)
	}
	if m := byM[time.October]; m.InCents != 0 || m.OutCents != 0 {
		t.Fatalf("oct = %+v", m)
	}
	if ov.TotalInCents != 1200 || ov.TotalOutCents != 900 || ov.BalanceCents != 300 || ov.Empty {
		t.Fatalf("totals = %+v", ov)
	}
	if !st.gotStart.Equal(p.Start) || !st.gotEnd.Equal(p.End) {
		t.Fatalf("store range = %v %v", st.gotStart, st.gotEnd)
	}
	if !st.gotMonthStart.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) ||
		!st.gotMonthEnd.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) || !st.got.Equal(now) {
		t.Fatalf("current args = %v %v %v", st.gotMonthStart, st.gotMonthEnd, st.got)
	}
	if ov.Current != st.cur || len(ov.Years) != 2 || ov.Years[0] != 2026 {
		t.Fatalf("current/years = %+v %v", ov.Current, ov.Years)
	}
}

func TestOverviewTodayIsUTC(t *testing.T) {
	// 2026-11-30 22:00 at UTC-3 is 2026-12-01 01:00 UTC: current month is December.
	west := time.FixedZone("west", -3*3600)
	now := time.Date(2026, 11, 30, 22, 0, 0, 0, west)
	st := &fakeOverviewStore{}
	if _, err := overviewSvc(st, now).Overview(context.Background(), "c1", domain.ParsePeriod("", now)); err != nil {
		t.Fatal(err)
	}
	if !st.gotMonthStart.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("monthStart = %v", st.gotMonthStart)
	}
}

func TestOverviewCategoryShares(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	rec, pay := domain.AccountTypeReceivable, domain.AccountTypePayable
	cs := domain.CategoriesFor(pay)
	cr := domain.CategoriesFor(rec)
	st := &fakeOverviewStore{cats: []CategoryTotal{
		{pay, cs[0], 100},
		{pay, cs[1], 300},
		{pay, cs[2], 0},
		{rec, cr[0], 500},
	}}
	ov, err := overviewSvc(st, now).Overview(context.Background(), "c1", domain.ParsePeriod("", now))
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.OutByCategory) != 2 || ov.OutByCategory[0].Category != cs[1] || ov.OutByCategory[1].Category != cs[0] {
		t.Fatalf("out = %+v", ov.OutByCategory)
	}
	if math.Abs(ov.OutByCategory[0].Percent-75) > 1e-9 || math.Abs(ov.OutByCategory[1].Percent-25) > 1e-9 {
		t.Fatalf("out pct = %+v", ov.OutByCategory)
	}
	if len(ov.InByCategory) != 1 || ov.InByCategory[0].Percent != 100 {
		t.Fatalf("in = %+v", ov.InByCategory)
	}
}

func TestOverviewEmpty(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	ov, err := overviewSvc(&fakeOverviewStore{}, now).Overview(context.Background(), "c1", domain.ParsePeriod("2024", now))
	if err != nil {
		t.Fatal(err)
	}
	if !ov.Empty || len(ov.Months) != 12 || len(ov.InByCategory) != 0 || len(ov.OutByCategory) != 0 {
		t.Fatalf("ov = %+v", ov)
	}
}

func TestOverviewStoreError(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	boom := errors.New("boom")
	_, err := overviewSvc(&fakeOverviewStore{err: boom}, now).Overview(context.Background(), "c1", domain.ParsePeriod("", now))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}
