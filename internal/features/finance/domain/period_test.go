package domain

import (
	"testing"
	"time"
)

func TestParsePeriod(t *testing.T) {
	utc := func(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }
	today := time.Date(2026, time.November, 14, 10, 0, 0, 0, time.UTC)
	// 2026-12-01 01:00 in UTC+3 is still 2026-11-30 22:00 UTC.
	east := time.FixedZone("east", 3*3600)
	eastToday := time.Date(2026, time.December, 1, 1, 0, 0, 0, east)
	// 2026-11-30 22:00 in UTC-3 is already 2026-12-01 01:00 UTC.
	west := time.FixedZone("west", -3*3600)
	westToday := time.Date(2026, time.November, 30, 22, 0, 0, 0, west)
	newYear := time.Date(2026, time.December, 31, 23, 0, 0, 0, west) // 2027-01-01 02:00 UTC

	cases := []struct {
		name       string
		raw        string
		today      time.Time
		kind       PeriodKind
		year       int
		start, end time.Time
		months     int
		param      string
	}{
		{"empty", "", today, PeriodLast12, 0, utc(2025, 12), utc(2026, 12), 12, ""},
		{"garbage", "abc", today, PeriodLast12, 0, utc(2025, 12), utc(2026, 12), 12, ""},
		{"too short", "202", today, PeriodLast12, 0, utc(2025, 12), utc(2026, 12), 12, ""},
		{"before 2000", "1999", today, PeriodLast12, 0, utc(2025, 12), utc(2026, 12), 12, ""},
		{"future", "2027", today, PeriodLast12, 0, utc(2025, 12), utc(2026, 12), 12, ""},
		{"past year", "2024", today, PeriodYear, 2024, utc(2024, 1), utc(2025, 1), 12, "2024"},
		{"year 2000", "2000", today, PeriodYear, 2000, utc(2000, 1), utc(2001, 1), 12, "2000"},
		{"current year", "2026", today, PeriodYear, 2026, utc(2026, 1), utc(2026, 12), 11, "2026"},
		{"current year january", "2026", time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), PeriodYear, 2026, utc(2026, 1), utc(2026, 2), 1, "2026"},
		{"non-UTC east", "", eastToday, PeriodLast12, 0, utc(2025, 12), utc(2026, 12), 12, ""},
		{"non-UTC west crosses month", "", westToday, PeriodLast12, 0, utc(2026, 1), utc(2027, 1), 12, ""},
		{"non-UTC west crosses year", "2027", newYear, PeriodYear, 2027, utc(2027, 1), utc(2027, 2), 1, "2027"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := ParsePeriod(c.raw, c.today)
			if p.Kind != c.kind || p.Year != c.year {
				t.Fatalf("kind/year = %s/%d, want %s/%d", p.Kind, p.Year, c.kind, c.year)
			}
			if !p.Start.Equal(c.start) || !p.End.Equal(c.end) {
				t.Fatalf("range = [%v, %v), want [%v, %v)", p.Start, p.End, c.start, c.end)
			}
			if p.Start.Location() != time.UTC || p.End.Location() != time.UTC {
				t.Fatalf("boundaries must be UTC")
			}
			ms := p.Months()
			if len(ms) != c.months {
				t.Fatalf("months = %d, want %d", len(ms), c.months)
			}
			if !ms[0].Equal(c.start) || !ms[len(ms)-1].AddDate(0, 1, 0).Equal(c.end) {
				t.Fatalf("months do not span the period: %v", ms)
			}
			if p.Param() != c.param {
				t.Fatalf("param = %q, want %q", p.Param(), c.param)
			}
		})
	}
}
