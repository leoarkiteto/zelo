package main

import (
	"testing"
	"time"
)

var receivableCategories = map[string]bool{
	"condo_fee": true, "fine_interest": true, "common_area_reservation": true, "extraordinary_income": true,
}

var payableCategories = map[string]bool{
	"maintenance": true, "cleaning": true, "utilities": true, "payroll": true,
	"third_party_services": true, "other": true,
}

func TestSeedHistoryCoversTwelveMonths(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 31, 23, 0, 0, 0, time.UTC),
	} {
		today := now.UTC().Truncate(24 * time.Hour)
		type totals struct{ in, out int64 }
		months := map[string]*totals{}
		for _, h := range seedHistory(now) {
			if h.amountCents <= 0 {
				t.Fatalf("non-positive amount: %+v", h)
			}
			if h.settlementDate.After(today) {
				t.Fatalf("settlement in the future: %+v", h)
			}
			key := h.settlementDate.Format("2006-01")
			if months[key] == nil {
				months[key] = &totals{}
			}
			switch h.accountType {
			case "receivable":
				if !receivableCategories[h.category] {
					t.Fatalf("invalid receivable category %q", h.category)
				}
				months[key].in += h.amountCents
			case "payable":
				if !payableCategories[h.category] {
					t.Fatalf("invalid payable category %q", h.category)
				}
				if h.unit != "" || h.paymentCode != "" {
					t.Fatalf("payable must not carry unit/payment code: %+v", h)
				}
				months[key].out += h.amountCents
			default:
				t.Fatalf("unknown account type %q", h.accountType)
			}
		}
		if len(months) != seedHistoryMonths {
			t.Fatalf("now=%s: got %d months, want %d", now.Format("2006-01-02"), len(months), seedHistoryMonths)
		}
		deficit := false
		for k, m := range months {
			if m.in == 0 || m.out == 0 {
				t.Errorf("month %s missing entradas or saídas: %+v", k, *m)
			}
			if m.out > m.in {
				deficit = true
			}
		}
		if !deficit {
			t.Errorf("now=%s: no month with saídas > entradas", now.Format("2006-01-02"))
		}
	}
}

// TestSeedNoDuplicateFeesAcrossBaseAndHistory combines the base fees with the
// history rows the seed actually inserts and rejects any repeated
// (unit, month) condo fee or payment code.
func TestSeedNoDuplicateFeesAcrossBaseAndHistory(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
	} {
		unitMonths := map[string]bool{}
		codes := map[string]bool{}
		add := func(unit, month, code string) {
			key := unit + "|" + month
			if unitMonths[key] {
				t.Errorf("now=%s: duplicate condo fee for %s", now.Format("2006-01-02"), key)
			}
			unitMonths[key] = true
			if codes[code] {
				t.Errorf("now=%s: duplicate payment code %s", now.Format("2006-01-02"), code)
			}
			codes[code] = true
		}
		for _, f := range seedFees {
			month := seedFeeDueDate(f, now).Format("2006-01")
			add(f.unit, month, seedFeePaymentCode(month, f.unit))
		}
		for _, h := range seedHistoryToInsert(now) {
			if h.accountType != "receivable" || h.category != "condo_fee" {
				continue
			}
			add(h.unit, h.dueDate.Format("2006-01"), h.paymentCode)
		}
	}
}

func TestSeedHistoryPaymentCodesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, h := range seedHistory(time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)) {
		if h.paymentCode == "" {
			continue
		}
		key := h.paymentCode
		if seen[key] {
			t.Fatalf("duplicate payment code %s", key)
		}
		seen[key] = true
	}
}
