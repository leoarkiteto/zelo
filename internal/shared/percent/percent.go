// Package percent parses and formats a percentage held as integer basis points
// (bp): 100 bp = 1%, 10000 bp = 100%. No floating point is involved.
package percent

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

// MaxBP is 100% in basis points.
const MaxBP = 10000

// ErrInvalid means the text is not a value in 0..100 with at most two decimals.
var ErrInvalid = errors.New("invalid percentage")

// ParseBP parses a percentage text ("2", "2,5" or "2.50") into basis points
// (200, 250, 250). It accepts 0..100 with a comma or dot separator, at most two
// decimals and at most three integer digits, and rejects anything else with
// ErrInvalid.
func ParseBP(s string) (int, error) {
	s = strings.Replace(strings.TrimSpace(s), ",", ".", 1)
	intPart, frac, _ := strings.Cut(s, ".")
	if (intPart == "" && frac == "") || len(frac) > 2 || len(intPart) > 3 ||
		!onlyDigits(intPart) || !onlyDigits(frac) {
		return 0, ErrInvalid
	}
	frac += strings.Repeat("0", 2-len(frac))
	whole := 0
	if intPart != "" {
		whole, _ = strconv.Atoi(intPart)
	}
	f, _ := strconv.Atoi(frac)
	bp := whole*100 + f
	if bp > MaxBP {
		return 0, ErrInvalid
	}
	return bp, nil
}

func onlyDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// FormatBP renders basis points as a percentage number without trailing zeros
// and without a "%" suffix: a comma decimal separator for Portuguese locales
// and a dot otherwise.
func FormatBP(bp int, lang i18n.Language) string {
	s := fmt.Sprintf("%d.%02d", bp/100, bp%100)
	s = strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
	if lang == i18n.LanguagePTBR || lang == i18n.LanguagePTPT {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}
