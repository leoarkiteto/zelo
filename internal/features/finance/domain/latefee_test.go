package domain

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestComputeLateFee(t *testing.T) {
	r := LateFeeRates{FeeBP: 200, InterestBP: 100}
	tests := []struct {
		name            string
		amount          int64
		due, settled    time.Time
		rates           LateFeeRates
		days            int
		fine, int_, tot int64
	}{
		{"AC-1 30 days", 100000, day(2026, 1, 1), day(2026, 1, 31), r, 30, 2000, 1000, 3000},
		{"AC-4 20 days", 38500, day(2026, 10, 25), day(2026, 11, 14), r, 20, 770, 257, 1027},
		{"AC-4 grace 5", 38500, day(2026, 10, 25), day(2026, 11, 14), LateFeeRates{200, 100, 5}, 15, 770, 193, 963},
		{"on time", 38500, day(2026, 10, 25), day(2026, 10, 25), r, 0, 0, 0, 0},
		{"before due", 38500, day(2026, 10, 25), day(2026, 10, 20), r, 0, 0, 0, 0},
		{"within grace", 38500, day(2026, 10, 25), day(2026, 10, 30), LateFeeRates{200, 100, 5}, 0, 0, 0, 0},
		{"time of day ignored", 38500, day(2026, 10, 25), time.Date(2026, 10, 25, 23, 59, 0, 0, time.UTC), r, 0, 0, 0, 0},
		{"zero amount", 0, day(2026, 10, 25), day(2026, 11, 14), r, 0, 0, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeLateFee(tc.amount, tc.due, tc.settled, tc.rates)
			want := LateFee{tc.days, tc.fine, tc.int_, tc.tot}
			if got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestAllocateEqual(t *testing.T) {
	shares := []ShareInput{
		{UnitID: "c", Code: "C", SharePPM: 1},
		{UnitID: "z", Code: "Z", SharePPM: 0},
		{UnitID: "a", Code: "A", SharePPM: 500000},
		{UnitID: "b", Code: "B", SharePPM: 7},
	}
	got := AllocateEqual(10000, shares)
	want := []int64{3333, 0, 3334, 3333}
	var sum int64
	for i, a := range got {
		if a.UnitID != shares[i].UnitID || a.BaseCents != want[i] || a.ReserveCents != 0 {
			t.Errorf("entry %d = %+v, want base %d", i, a, want[i])
		}
		sum += a.BaseCents
	}
	if sum != 10000 {
		t.Errorf("sum = %d", sum)
	}
	for _, a := range AllocateEqual(100, []ShareInput{{Code: "A"}}) {
		if a.BaseCents != 0 {
			t.Errorf("no positive share must allocate 0, got %d", a.BaseCents)
		}
	}
}

func TestSplitInstallments(t *testing.T) {
	for _, p := range SplitInstallments(1200000, 6) {
		if p != 200000 {
			t.Errorf("installment = %d, want 200000", p)
		}
	}
	got := SplitInstallments(10000, 3)
	if got[0] != 3334 || got[1] != 3333 || got[2] != 3333 {
		t.Errorf("got %v", got)
	}
}

func TestInstallmentDueDate(t *testing.T) {
	first := day(2026, 11, 10)
	want := []time.Time{day(2026, 11, 10), day(2026, 12, 10), day(2027, 1, 10), day(2027, 2, 10), day(2027, 11, 10)}
	for i, n := range []int{1, 2, 3, 4, 13} {
		if got := InstallmentDueDate(first, n); !got.Equal(want[i]) {
			t.Errorf("n=%d: got %v, want %v", n, got, want[i])
		}
	}
	if got := InstallmentDueDate(day(2026, 1, 28), 2); !got.Equal(day(2026, 2, 28)) {
		t.Errorf("got %v", got)
	}
}
