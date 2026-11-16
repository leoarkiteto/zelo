// Package permille parses and formats a condominium share (Permilagem) held as
// integer parts per million (ppm): 1000 per mille = 100% = 1,000,000 ppm.
// No floating point is involved.
package permille

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

// Total is the share total of a condominium: 1000 per mille = 100% = 1,000,000 ppm.
const Total = 1_000_000

// ErrInvalid means the text is not a value in 0..1000 with at most three decimals.
var ErrInvalid = errors.New("invalid permille")

// Parse parses a per mille text ("45,5" or "45.5") into ppm (45500). It accepts
// 0..1000 with up to three decimals and rejects anything else with ErrInvalid.
func Parse(s string) (int, error) {
	s = strings.Replace(strings.TrimSpace(s), ",", ".", 1)
	intPart, frac, _ := strings.Cut(s, ".")
	if (intPart == "" && frac == "") || len(frac) > 3 || len(intPart) > 4 ||
		!digitsOnly(intPart) || !digitsOnly(frac) {
		return 0, ErrInvalid
	}
	frac += strings.Repeat("0", 3-len(frac))
	whole := 0
	if intPart != "" {
		whole, _ = strconv.Atoi(intPart)
	}
	f, _ := strconv.Atoi(frac)
	ppm := whole*1000 + f
	if ppm > Total {
		return 0, ErrInvalid
	}
	return ppm, nil
}

func digitsOnly(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Format renders ppm as per mille without trailing zeros, with a comma decimal
// separator for Portuguese locales and a dot otherwise. A negative value keeps
// its sign.
func Format(ppm int, lang i18n.Language) string {
	sign := ""
	if ppm < 0 {
		sign, ppm = "-", -ppm
	}
	s := fmt.Sprintf("%d.%03d", ppm/1000, ppm%1000)
	s = strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
	if lang == i18n.LanguagePTBR || lang == i18n.LanguagePTPT {
		s = strings.Replace(s, ".", ",", 1)
	}
	return sign + s
}
