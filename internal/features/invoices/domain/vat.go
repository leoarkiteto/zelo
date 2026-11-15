package domain

// AllowedRates returns the VAT rates, in basis points, a Linha da fatura may
// use with TreatmentRate in the given fiscal region (FR-12). Continental is
// 6/13/23 %, Madeira 4/12/22 %, Açores 4/9/16 %. An unknown region has none.
func AllowedRates(region FiscalRegion) []int {
	switch region {
	case RegionContinental:
		return []int{600, 1300, 2300}
	case RegionMadeira:
		return []int{400, 1200, 2200}
	case RegionAcores:
		return []int{400, 900, 1600}
	}
	return nil
}

// RateAllowed reports whether bp is an allowed rate for the region.
func RateAllowed(region FiscalRegion, bp int) bool {
	for _, r := range AllowedRates(region) {
		if r == bp {
			return true
		}
	}
	return false
}

// ExemptionCode is an official AT VAT exemption reason code (motivo de
// isenção) with the i18n key of its description.
type ExemptionCode struct {
	Code           string
	DescriptionKey string
}

// exemptionCodes pins the official AT list as of 2026:
// M01-M16, M19-M26, M30-M46 and M99.
var exemptionCodes = buildExemptionCodes()

func buildExemptionCodes() []ExemptionCode {
	nums := []int{}
	add := func(from, to int) {
		for n := from; n <= to; n++ {
			nums = append(nums, n)
		}
	}
	add(1, 16)
	add(19, 26)
	add(30, 46)
	add(99, 99)
	out := make([]ExemptionCode, 0, len(nums))
	for _, n := range nums {
		code := "M" + twoDigits(n)
		out = append(out, ExemptionCode{Code: code, DescriptionKey: "invoices.exemption." + code})
	}
	return out
}

func twoDigits(n int) string {
	return string([]byte{byte('0' + n/10), byte('0' + n%10)})
}

// ExemptionCodes returns the pinned official AT exemption codes, in order.
func ExemptionCodes() []ExemptionCode {
	out := make([]ExemptionCode, len(exemptionCodes))
	copy(out, exemptionCodes)
	return out
}

// ValidExemptionCode reports whether code is in the pinned AT list.
func ValidExemptionCode(code string) bool {
	for _, c := range exemptionCodes {
		if c.Code == code {
			return true
		}
	}
	return false
}
