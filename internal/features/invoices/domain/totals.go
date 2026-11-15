package domain

import (
	"fmt"
	"sort"
	"strings"
)

// Computation limits that keep every intermediate product inside int64.
const (
	MaxLines          = 500
	maxQuantityMilli  = 1_000_000_000     // 1,000,000 units
	maxUnitPriceCents = 1_000_000_000     // 10,000,000.00
	maxLineBaseCents  = 1_000_000_000_000 // 10,000,000,000.00
)

// IsProductLine reports whether the line is a compra de produtos line, whose
// base is computed from quantity, unit price and discount. A line is a
// product line when any of QuantityMilli, UnitPriceCents or DiscountCents is
// non-zero; otherwise BaseCents is the amount given by the user.
func IsProductLine(l Line) bool {
	return l.QuantityMilli != 0 || l.UnitPriceCents != 0 || l.DiscountCents != 0
}

// roundDivHalfUp returns n/d rounded half-up for n >= 0 and d > 0.
func roundDivHalfUp(n, d int64) int64 { return (n + d/2) / d }

// LineGross is the line amount before discount: round_half_up(qty × unit
// price / 1000) for product lines, BaseCents otherwise. Out-of-range inputs
// are rejected by LineBase; LineGross assumes valid ones.
func LineGross(l Line) int64 {
	if !IsProductLine(l) {
		return l.BaseCents
	}
	return roundDivHalfUp(l.QuantityMilli*l.UnitPriceCents, 1000)
}

// LineBase computes the line base (taxable amount). For product lines it is
// round_half_up(qty_milli × unit_price_cents / 1000) − discount_cents; for
// other lines it is BaseCents. Errors carry an i18n key (use ErrorKey):
// ErrInvalidQuantity for qty <= 0 on a product line, ErrInvalidAmount for a
// negative price or discount, KeyAmountTooLarge, and KeyNegativeBase when
// the base would be negative (discount above qty × price).
func LineBase(l Line) (int64, error) {
	if !IsProductLine(l) {
		if l.BaseCents < 0 {
			return 0, newKeyError(KeyNegativeBase)
		}
		if l.BaseCents > maxLineBaseCents {
			return 0, newKeyError(KeyAmountTooLarge)
		}
		return l.BaseCents, nil
	}
	if l.QuantityMilli <= 0 {
		return 0, ErrInvalidQuantity
	}
	if l.UnitPriceCents < 0 || l.DiscountCents < 0 {
		return 0, ErrInvalidAmount
	}
	if l.QuantityMilli > maxQuantityMilli || l.UnitPriceCents > maxUnitPriceCents || l.DiscountCents > maxLineBaseCents {
		return 0, newKeyError(KeyAmountTooLarge)
	}
	gross := LineGross(l)
	base := gross - l.DiscountCents
	if base < 0 {
		return 0, newKeyError(KeyNegativeBase)
	}
	if gross > maxLineBaseCents {
		return 0, newKeyError(KeyAmountTooLarge)
	}
	return base, nil
}

func lineField(i int, f string) string { return fmt.Sprintf("lines[%d].%s", i, f) }

// ComputeTotals computes every total from the lines (FR-13, FR-14; NFR-1).
//
//   - Gross is the sum of the line amounts before discount; DiscountCents is
//     the sum of the line discounts; BaseCents = gross − discounts.
//   - Taxes has one bucket per (treatment, rate): the base is the sum of the
//     line bases and, for rate buckets, VAT = round_half_up(base × bp / 10000).
//     Exempt and reverse-charge buckets always have VAT 0. Order: rate
//     buckets by descending rate, then exempt, then reverse charge.
//   - TotalCents = bases + VAT; PayableCents = total − withholding.
//
// A rate line must use a rate from AllowedRates(region); an exempt line needs
// a code from ExemptionCodes and a non-blank reason; a negative line base is
// rejected. The withholding must satisfy 0 ≤ withholding ≤ total. Failures
// are returned as FieldErrors keyed by the form input names ("lines[i].base",
// "lines[i].discount" for the amount errors of a product line, "lines[i].rate",
// "lines[i].exemption_code", "lines[i].exemption_reason", "lines[i].qty",
// "lines[i].treatment", "fiscal_region", "lines", "withholding") and Totals is
// then the zero value. No lines yields zero totals (valid for drafts).
func ComputeTotals(region FiscalRegion, lines []Line, withholdingCents int64) (Totals, error) {
	errs := FieldErrors{}
	if !region.Valid() {
		errs.Add("fiscal_region", KeyInvalid)
	}
	if len(lines) > MaxLines {
		errs.Add("lines", KeyTooManyLines)
		return Totals{}, errs
	}

	type key struct {
		t  VATTreatment
		bp int
	}
	buckets := map[key]*TaxBucket{}
	var t Totals

	for i, l := range lines {
		if !l.Treatment.Valid() {
			errs.Add(lineField(i, "treatment"), KeyUnknownTreatment)
			continue
		}
		base, err := LineBase(l)
		if err != nil {
			// Keys are the names of the form inputs: "qty" for the quantity
			// and "discount" for any other amount error of a product line.
			field := "base"
			if IsProductLine(l) {
				field = "discount"
				if l.QuantityMilli <= 0 {
					field = "qty"
				}
			}
			errs.Add(lineField(i, field), ErrorKey(err))
			continue
		}
		bp := 0
		switch l.Treatment {
		case TreatmentRate:
			if region.Valid() && !RateAllowed(region, l.RateBP) {
				errs.Add(lineField(i, "rate"), KeyRateNotAllowed)
				continue
			}
			bp = l.RateBP
		case TreatmentExempt:
			code := strings.TrimSpace(l.ExemptionCode)
			switch {
			case code == "":
				errs.Add(lineField(i, "exemption_code"), KeyExemptionCodeRequired)
			case !ValidExemptionCode(code):
				errs.Add(lineField(i, "exemption_code"), KeyExemptionCodeInvalid)
			}
			if strings.TrimSpace(l.ExemptionReason) == "" {
				errs.Add(lineField(i, "exemption_reason"), KeyExemptionReasonMissing)
			}
		}
		k := key{l.Treatment, bp}
		b := buckets[k]
		if b == nil {
			b = &TaxBucket{Treatment: l.Treatment, RateBP: bp}
			buckets[k] = b
		}
		b.BaseCents += base
		t.GrossCents += LineGross(l)
		if IsProductLine(l) {
			t.DiscountCents += l.DiscountCents
		}
	}
	if len(errs) > 0 {
		return Totals{}, errs
	}

	for _, b := range buckets {
		if b.Treatment == TreatmentRate {
			b.VATCents = roundDivHalfUp(b.BaseCents*int64(b.RateBP), 10000)
		}
		t.BaseCents += b.BaseCents
		t.VATCents += b.VATCents
		t.Taxes = append(t.Taxes, *b)
	}
	order := map[VATTreatment]int{TreatmentRate: 0, TreatmentExempt: 1, TreatmentReverseCharge: 2}
	sort.Slice(t.Taxes, func(i, j int) bool {
		a, b := t.Taxes[i], t.Taxes[j]
		if a.Treatment != b.Treatment {
			return order[a.Treatment] < order[b.Treatment]
		}
		return a.RateBP > b.RateBP
	})
	t.TotalCents = t.BaseCents + t.VATCents

	if withholdingCents < 0 || withholdingCents > t.TotalCents {
		return Totals{}, FieldErrors{"withholding": KeyWithholdingRange}
	}
	t.WithholdingCents = withholdingCents
	t.PayableCents = t.TotalCents - withholdingCents
	return t, nil
}
