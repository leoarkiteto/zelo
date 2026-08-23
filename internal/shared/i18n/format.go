package i18n

import (
	"fmt"
	"time"
)

// ptBRWeekday maps Go weekdays to Brazilian Portuguese names.
var ptBRWeekday = map[time.Weekday]string{
	time.Sunday:    "domingo",
	time.Monday:    "segunda-feira",
	time.Tuesday:   "terça-feira",
	time.Wednesday: "quarta-feira",
	time.Thursday:  "quinta-feira",
	time.Friday:    "sexta-feira",
	time.Saturday:  "sábado",
}

// ptBRMonth maps Go months to Brazilian Portuguese names.
var ptBRMonth = map[time.Month]string{
	time.January:   "janeiro",
	time.February:  "fevereiro",
	time.March:     "março",
	time.April:     "abril",
	time.May:       "maio",
	time.June:      "junho",
	time.July:      "julho",
	time.August:    "agosto",
	time.September: "setembro",
	time.October:   "outubro",
	time.November:  "novembro",
	time.December:  "dezembro",
}

// FormatDate renders a date using the locale's display convention (FR-010).
func FormatDate(lang Language, t time.Time) string {
	switch lang {
	case LanguagePTBR:
		return t.Format("02/01/2006")
	default:
		return t.Format("January 2, 2006")
	}
}

// FormatLongDate renders a full date with weekday and month names in the
// locale's convention (FR-010).
func FormatLongDate(lang Language, t time.Time) string {
	switch lang {
	case LanguagePTBR:
		return ptBRWeekday[t.Weekday()] + ", " + t.Format("02") + " de " +
			ptBRMonth[t.Month()] + " de " + t.Format("2006")
	default:
		return t.Format("Monday, January 2, 2006")
	}
}

// FormatNumber renders an integer using the locale's convention (FR-010).
// Both supported locales use the same integer rendering today; the function
// exists as the single seam for number formatting.
func FormatNumber(lang Language, n int) string {
	return fmt.Sprintf("%d", n)
}
