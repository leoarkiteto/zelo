package domain

import (
	"testing"
	"time"
)

func sum(xs []int64) (t int64) {
	for _, x := range xs {
		t += x
	}
	return
}

func TestSplitAcrossPeriods(t *testing.T) {
	got := SplitAcrossPeriods(1000000, 12) // AC-6
	if len(got) != 12 || sum(got) != 1000000 {
		t.Fatalf("got %v", got)
	}
	for i, v := range got {
		want := int64(83333)
		if i < 4 {
			want = 83334
		}
		if v != want {
			t.Errorf("period %d = %d, want %d", i+1, v, want)
		}
	}
	for _, c := range []struct {
		total int64
		n     int
	}{{0, 4}, {1, 4}, {3, 12}, {1000001, 4}, {7, 1}} {
		if s := sum(SplitAcrossPeriods(c.total, c.n)); s != c.total {
			t.Errorf("sum(%d,%d) = %d", c.total, c.n, s)
		}
	}
	if SplitAcrossPeriods(100, 0) != nil || SplitAcrossPeriods(-1, 4) != nil {
		t.Error("expected nil for invalid input")
	}
}

func TestPeriodicityPeriods(t *testing.T) {
	if PeriodicityMonthly.Periods() != 12 || PeriodicityQuarterly.Periods() != 4 || Periodicity("x").Periods() != 0 {
		t.Error("unexpected periods")
	}
}

func TestReserveFor(t *testing.T) {
	cases := []struct {
		base int64
		bp   int
		want int64
	}{
		{100000, 1000, 10000},
		{1, 5000, 1},    // 0.5 -> half up
		{1, 4999, 0},    // 0.4999
		{333, 1000, 33}, // 33.3
		{335, 1000, 34}, // 33.5 -> 34
		{100000, 0, 0},
		{0, 1000, 0},
		{100000, 10000, 100000},
	}
	for _, c := range cases {
		if got := ReserveFor(c.base, c.bp); got != c.want {
			t.Errorf("ReserveFor(%d,%d) = %d, want %d", c.base, c.bp, got, c.want)
		}
	}
}

func TestAllocateByShareAC7(t *testing.T) {
	in := []ShareInput{{"a", "A", 400000}, {"b", "B", 350000}, {"c", "C", 250000}}
	got := AllocateByShare(100000, ReserveFor(100000, 1000), in)
	want := []int64{44000, 38500, 27500}
	for i, a := range got {
		if a.TotalCents() != want[i] {
			t.Errorf("%s quota = %d, want %d", a.Code, a.TotalCents(), want[i])
		}
	}
}

func TestAllocateByShareSums(t *testing.T) {
	in := []ShareInput{{"a", "A", 333333}, {"b", "B", 333333}, {"c", "C", 333334}}
	got := AllocateByShare(100000, 10000, in)
	var b, r int64
	for _, a := range got {
		b += a.BaseCents
		r += a.ReserveCents
	}
	if b != 100000 || r != 10000 {
		t.Fatalf("sums base=%d reserve=%d", b, r)
	}
}

func TestAllocateByShareTieBreakLowestCode(t *testing.T) {
	// 100 cents over three equal shares: 33 each + 1 leftover -> lowest code.
	in := []ShareInput{{"u3", "C", 333333}, {"u1", "A", 333333}, {"u2", "B", 333333}}
	got := AllocateByShare(100, 0, in)
	byCode := map[string]int64{}
	for _, a := range got {
		byCode[a.Code] = a.BaseCents
	}
	if byCode["A"] != 34 || byCode["B"] != 33 || byCode["C"] != 33 {
		t.Fatalf("got %v", byCode)
	}
	if got[0].Code != "C" || got[1].Code != "A" {
		t.Error("input order must be preserved")
	}
}

func TestAllocateByShareLargestRemainder(t *testing.T) {
	// 10 cents: 50%/30%/20% exact; 1 cent over 3 units, remainder decides.
	in := []ShareInput{{"a", "A", 100000}, {"b", "B", 600000}, {"c", "C", 300000}}
	got := AllocateByShare(1, 0, in)
	if got[1].BaseCents != 1 || got[0].BaseCents != 0 || got[2].BaseCents != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestAllocateByShareZeroShares(t *testing.T) {
	in := []ShareInput{{"a", "A", 0}, {"b", "B", 1000000}, {"c", "C", 0}}
	got := AllocateByShare(1234, 56, in)
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].TotalCents() != 0 || got[2].TotalCents() != 0 || got[1].BaseCents != 1234 || got[1].ReserveCents != 56 {
		t.Fatalf("got %+v", got)
	}
	none := AllocateByShare(100, 10, []ShareInput{{"a", "A", 0}})
	if none[0].TotalCents() != 0 {
		t.Error("no positive share must allocate nothing")
	}
}

func TestAllocateByShareLarge(t *testing.T) {
	in := []ShareInput{{"a", "A", 500000}, {"b", "B", 500000}}
	got := AllocateByShare(9_000_000_000_000_001, 0, in) // would overflow int64 product
	if got[0].BaseCents+got[1].BaseCents != 9_000_000_000_000_001 {
		t.Fatalf("got %+v", got)
	}
}

func TestDueDate(t *testing.T) {
	cases := []struct {
		year int
		p    Periodicity
		idx  int
		day  int
		want time.Time
	}{
		{2026, PeriodicityMonthly, 12, 8, time.Date(2026, 12, 8, 0, 0, 0, 0, time.UTC)},
		{2026, PeriodicityQuarterly, 2, 15, time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)},
		{2026, PeriodicityQuarterly, 4, 1, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		{2026, PeriodicityMonthly, 2, 28, time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)},
		{2028, PeriodicityMonthly, 2, 28, time.Date(2028, 2, 28, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		if got := DueDate(c.year, c.p, c.idx, c.day); !got.Equal(c.want) {
			t.Errorf("DueDate(%d,%s,%d,%d) = %v, want %v", c.year, c.p, c.idx, c.day, got, c.want)
		}
	}
}

func TestPeriodLabel(t *testing.T) {
	if PeriodLabel(2026, PeriodicityMonthly, 3) != "2026-03" || PeriodLabel(2026, PeriodicityQuarterly, 2) != "2026-T2" {
		t.Error("unexpected label")
	}
}

func TestStatePredicates(t *testing.T) {
	if !BudgetDraft.CanEdit() || BudgetApproved.CanEdit() || !BudgetDraft.CanApprove() || !BudgetApproved.CanReopen() || BudgetDraft.CanReopen() {
		t.Error("budget predicates")
	}
	if !IssuanceIssued.CanCancel() || IssuanceCanceled.CanCancel() {
		t.Error("issuance predicates")
	}
}
