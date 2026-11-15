package domain

import (
	"strings"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func commaDecimal(lang i18n.Language) bool {
	return lang == i18n.LanguagePTBR || lang == i18n.LanguagePTPT
}

// ParseQuantityMilli parses a user-typed quantity into milli-units with up to
// 3 decimals ("2.5" -> 2500), without floating point.
//
// en uses "." as decimal and "," as thousands separator. pt-br and pt-pt use
// "," as decimal and "." (or a space) as thousands separator; as a leniency
// for pt, a single "." that is not followed by exactly 3 digits and no ","
// is read as a decimal point ("2.5" -> 2500, but "1.234" -> 1234). Negative
// numbers, signs and more than 3 decimals return ErrInvalidQuantity. Zero
// parses successfully; positivity is checked by ComputeTotals.
func ParseQuantityMilli(lang i18n.Language, s string) (int64, error) {
	s = strings.TrimSpace(s)
	for _, sp := range []string{" ", " ", " "} {
		s = strings.ReplaceAll(s, sp, "")
	}
	if s == "" {
		return 0, ErrInvalidQuantity
	}
	dec, thou := byte('.'), byte(',')
	if commaDecimal(lang) {
		dec, thou = ',', '.'
		if strings.IndexByte(s, ',') < 0 && strings.Count(s, ".") == 1 {
			if len(s)-strings.IndexByte(s, '.')-1 != 3 {
				dec, thou = '.', 0
			}
		}
	}
	if strings.Count(s, string(dec)) > 1 {
		return 0, ErrInvalidQuantity
	}
	intPart, frac := s, ""
	if i := strings.IndexByte(s, dec); i >= 0 {
		intPart, frac = s[:i], s[i+1:]
	}
	if len(frac) > 3 || (intPart == "" && frac == "") {
		return 0, ErrInvalidQuantity
	}
	if thou != 0 && strings.IndexByte(intPart, thou) >= 0 {
		groups := strings.Split(intPart, string(thou))
		if len(groups[0]) < 1 || len(groups[0]) > 3 {
			return 0, ErrInvalidQuantity
		}
		for _, g := range groups[1:] {
			if len(g) != 3 {
				return 0, ErrInvalidQuantity
			}
		}
		intPart = strings.Join(groups, "")
	}
	if len(intPart) > 12 {
		return 0, ErrInvalidQuantity
	}
	var n int64
	for _, part := range []string{intPart, (frac + "000")[:3]} {
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return 0, ErrInvalidQuantity
			}
		}
	}
	for i := 0; i < len(intPart); i++ {
		n = n*10 + int64(intPart[i]-'0')
	}
	n *= 1000
	f := (frac + "000")[:3]
	for i := 0; i < 3; i++ {
		n += int64(f[i]-'0') * []int64{100, 10, 1}[i]
	}
	return n, nil
}
