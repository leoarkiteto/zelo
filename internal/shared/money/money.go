// Package money formats and parses monetary amounts held as integer cents,
// and carries the active condominium's currency through the request context.
// No floating-point arithmetic is used anywhere (NFR-1).
package money

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

// Currency is an ISO 4217 currency code supported by the app.
type Currency string

const (
	// EUR is the euro.
	EUR Currency = "EUR"
	// BRL is the Brazilian real, the default currency.
	BRL Currency = "BRL"
)

// nbsp is U+00A0, used by pt-PT EUR formatting.
const nbsp = " "

// Valid reports whether c is a supported currency.
func (c Currency) Valid() bool { return c == EUR || c == BRL }

// Default returns the default currency, BRL.
func Default() Currency { return BRL }

// Format renders integer cents as a localized amount in currency c.
func Format(lang i18n.Language, c Currency, cents int64) string {
	sign := ""
	// Work with an unsigned magnitude so math.MinInt64 does not overflow.
	var mag uint64
	if cents < 0 {
		sign = "-"
		mag = uint64(-(cents + 1)) + 1
	} else {
		mag = uint64(cents)
	}
	whole := mag / 100
	frac := mag % 100
	fracS := strconv.FormatUint(frac, 10)
	if len(fracS) < 2 {
		fracS = "0" + fracS
	}

	comma := lang == i18n.LanguagePTBR || lang == i18n.LanguagePTPT
	var num string
	switch {
	case lang == i18n.LanguagePTPT && c == EUR:
		num = group(whole, nbsp) + "," + fracS
	case comma:
		num = group(whole, ".") + "," + fracS
	default:
		num = group(whole, ",") + "." + fracS
	}

	switch c {
	case EUR:
		switch lang {
		case i18n.LanguagePTPT:
			return sign + num + nbsp + "€"
		case i18n.LanguagePTBR:
			return sign + "€ " + num
		default:
			return sign + "€" + num
		}
	default:
		return sign + "R$ " + num
	}
}

func group(n uint64, sep string) string {
	s := strconv.FormatUint(n, 10)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// ErrInvalidAmount is returned by ParseCents for unparseable amounts.
var ErrInvalidAmount = errors.New("invalid amount")

// maxWholeDigits keeps whole*100 + frac within int64.
const maxWholeDigits = 16

// ParseCents parses a user-typed non-negative amount into integer cents using
// the language's separators (pt-br/pt-pt: "1.234,56"; en: "1,234.56"). At
// most two decimals are accepted; thousands separators are optional but, when
// present, must group by three.
func ParseCents(lang i18n.Language, s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrInvalidAmount
	}
	decSep, groupSeps := byte('.'), ","
	if lang == i18n.LanguagePTBR || lang == i18n.LanguagePTPT {
		decSep, groupSeps = ',', "."
		if lang == i18n.LanguagePTPT {
			groupSeps = ".  "
		}
	}

	intPart, fracPart := s, ""
	if i := strings.IndexByte(s, decSep); i >= 0 {
		intPart, fracPart = s[:i], s[i+1:]
		if fracPart == "" || len(fracPart) > 2 || !allDigits(fracPart) {
			return 0, ErrInvalidAmount
		}
	}
	if intPart == "" {
		return 0, ErrInvalidAmount
	}

	var digits strings.Builder
	if strings.ContainsAny(intPart, groupSeps) {
		// Grouped form: first group 1-3 digits, then groups of exactly 3,
		// all using the same separator.
		sep := rune(0)
		for _, r := range intPart {
			if strings.ContainsRune(groupSeps, r) {
				sep = r
				break
			}
		}
		parts := strings.Split(intPart, string(sep))
		for i, p := range parts {
			if !allDigits(p) {
				return 0, ErrInvalidAmount
			}
			if i == 0 && (len(p) < 1 || len(p) > 3) {
				return 0, ErrInvalidAmount
			}
			if i > 0 && len(p) != 3 {
				return 0, ErrInvalidAmount
			}
			digits.WriteString(p)
		}
	} else {
		if !allDigits(intPart) {
			return 0, ErrInvalidAmount
		}
		digits.WriteString(intPart)
	}
	if digits.Len() > maxWholeDigits {
		return 0, ErrInvalidAmount
	}
	whole, err := strconv.ParseInt(digits.String(), 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	var frac int64
	if fracPart != "" {
		if len(fracPart) == 1 {
			fracPart += "0"
		}
		frac, err = strconv.ParseInt(fracPart, 10, 64)
		if err != nil {
			return 0, ErrInvalidAmount
		}
	}
	return whole*100 + frac, nil
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

type ctxKey struct{}

// WithCurrency returns a context carrying currency c.
func WithCurrency(ctx context.Context, c Currency) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// From returns the context's currency, or Default() when unset or invalid.
func From(ctx context.Context) Currency {
	if c, ok := ctx.Value(ctxKey{}).(Currency); ok && c.Valid() {
		return c
	}
	return Default()
}
