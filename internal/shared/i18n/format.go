package i18n

import (
	"fmt"
	"strings"
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

// ptPTWeekday maps Go weekdays to European Portuguese names.
var ptPTWeekday = map[time.Weekday]string{
	time.Sunday:    "domingo",
	time.Monday:    "segunda-feira",
	time.Tuesday:   "terça-feira",
	time.Wednesday: "quarta-feira",
	time.Thursday:  "quinta-feira",
	time.Friday:    "sexta-feira",
	time.Saturday:  "sábado",
}

// ptPTMonth maps Go months to European Portuguese names.
var ptPTMonth = map[time.Month]string{
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
	case LanguagePTBR, LanguagePTPT:
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
	case LanguagePTPT:
		return ptPTWeekday[t.Weekday()] + ", " + t.Format("02") + " de " +
			ptPTMonth[t.Month()] + " de " + t.Format("2006")
	default:
		return t.Format("Monday, January 2, 2006")
	}
}

// FormatNumber renders an integer using the locale's convention (FR-010).
// pt-pt groups thousands with a no-break space (U+00A0); en and pt-br render
// plain digits. The function is the single seam for number formatting.
func FormatNumber(lang Language, n int) string {
	if lang != LanguagePTPT {
		return fmt.Sprintf("%d", n)
	}
	neg := n < 0
	digits := fmt.Sprintf("%d", n)
	if neg {
		digits = digits[1:]
	}
	var b strings.Builder
	if neg {
		b.WriteString("-")
	}
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteString(" ")
		}
		b.WriteRune(r)
	}
	return b.String()
}
