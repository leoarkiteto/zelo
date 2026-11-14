package domain

import (
	"strconv"
	"time"
)

// PeriodKind identifies how a Period was chosen.
type PeriodKind string

const (
	// PeriodLast12 is the 12 months ending in the current month.
	PeriodLast12 PeriodKind = "last12"
	// PeriodYear is a specific calendar year (cut at the current month).
	PeriodYear PeriodKind = "year"
)

// Period is a month-aligned half-open interval [Start, End), UTC.
type Period struct {
	Kind  PeriodKind
	Year  int       // set when Kind == PeriodYear
	Start time.Time // first day of the first month
	End   time.Time // first day of the month after the last month
}

func monthStart(y int, m time.Month) time.Time {
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

// ParsePeriod parses the period query value. Empty or invalid input, or a year
// before 2000 or after today's year, yields the last 12 months ending in
// today's month. "YYYY" yields Jan..Dec, cut at today's month for the current
// year. today is converted to UTC first.
func ParsePeriod(raw string, today time.Time) Period {
	t := today.UTC()
	cur := monthStart(t.Year(), t.Month())
	if len(raw) == 4 {
		if y, err := strconv.Atoi(raw); err == nil && y >= 2000 && y <= t.Year() {
			end := monthStart(y+1, time.January)
			if y == t.Year() {
				end = cur.AddDate(0, 1, 0)
			}
			return Period{Kind: PeriodYear, Year: y, Start: monthStart(y, time.January), End: end}
		}
	}
	return Period{
		Kind:  PeriodLast12,
		Start: cur.AddDate(0, -11, 0),
		End:   cur.AddDate(0, 1, 0),
	}
}

// Months returns the first day of each month in [Start, End).
func (p Period) Months() []time.Time {
	var out []time.Time
	for m := p.Start; m.Before(p.End); m = m.AddDate(0, 1, 0) {
		out = append(out, m)
	}
	return out
}

// Param returns the query value: "" for last12, "YYYY" for a year.
func (p Period) Param() string {
	if p.Kind == PeriodYear {
		return strconv.Itoa(p.Year)
	}
	return ""
}
