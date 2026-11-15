package domain

import "strings"

// CountryPT is the country code that triggers NIF validation (FR-6).
const CountryPT = "PT"

func isPT(country string) bool { return strings.EqualFold(strings.TrimSpace(country), CountryPT) }

// NormalizeNIF prepares a fornecedor tax id for validation and storage. The
// id is always trimmed. Only when the country is PT are all whitespace removed
// and an optional "PT" prefix stripped, so "PT 123 456 789" becomes
// "123456789". A foreign tax id is free text and is only trimmed.
func NormalizeNIF(country, s string) string {
	s = strings.TrimSpace(s)
	if !isPT(country) {
		return s
	}
	s = strings.Join(strings.Fields(s), "")
	if len(s) >= 2 && strings.EqualFold(s[:2], "PT") {
		s = s[2:]
	}
	return s
}

// ValidateNIF checks a tax id exactly as given (normalize it first with
// NormalizeNIF). For country PT it must be 9 digits with a valid mod-11 check
// digit, otherwise ErrInvalidNIF is returned. For any other country the id is
// free text and always valid, including empty.
func ValidateNIF(country, nif string) error {
	if !isPT(country) {
		return nil
	}
	if len(nif) != 9 {
		return ErrInvalidNIF
	}
	sum := 0
	for i := 0; i < 9; i++ {
		c := nif[i]
		if c < '0' || c > '9' {
			return ErrInvalidNIF
		}
		if i < 8 {
			sum += int(c-'0') * (9 - i)
		}
	}
	check := 11 - sum%11
	if check >= 10 {
		check = 0
	}
	if int(nif[8]-'0') != check {
		return ErrInvalidNIF
	}
	return nil
}

// NormalizeNumber normalizes an invoice number before the duplicate check
// (FR-23): it is trimmed and internal whitespace runs collapse to one space.
func NormalizeNumber(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
