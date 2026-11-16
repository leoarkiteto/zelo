package templates

import (
	"testing"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func TestPeriodTextAndQuotaTitlePerLocale(t *testing.T) {
	monthly := domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityMonthly, Index: 12, UnitCode: "A"}
	quarterly := domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityQuarterly, Index: 2, UnitCode: "A"}
	for _, tt := range []struct {
		lang                       i18n.Language
		monthly, quarterly, titleQ string
	}{
		{i18n.LanguageEN, "December 2026", "Q2 2026", "Condo fee Q2 2026 — Unit A"},
		{i18n.LanguagePTBR, "dezembro de 2026", "2º trimestre de 2026", "Taxa condominial 2º trimestre de 2026 — Unidade A"},
		{i18n.LanguagePTPT, "dezembro de 2026", "2.º trimestre de 2026", "Quota 2.º trimestre de 2026 — Unidade A"},
	} {
		if got := QuotaPeriodText(tt.lang, monthly); got != tt.monthly {
			t.Errorf("%s monthly = %q, want %q", tt.lang, got, tt.monthly)
		}
		if got := QuotaPeriodText(tt.lang, quarterly); got != tt.quarterly {
			t.Errorf("%s quarterly = %q, want %q", tt.lang, got, tt.quarterly)
		}
		if got := QuotaTitle(tt.lang, quarterly); got != tt.titleQ {
			t.Errorf("%s title = %q, want %q", tt.lang, got, tt.titleQ)
		}
	}
	for m := 1; m <= 12; m++ {
		p := domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityMonthly, Index: m}
		for _, l := range []i18n.Language{i18n.LanguageEN, i18n.LanguagePTBR, i18n.LanguagePTPT} {
			if got := QuotaPeriodText(l, p); got == "" || got[0] == 'q' && got[1] == 'u' {
				t.Errorf("%s month %d = %q (missing key?)", l, m, got)
			}
		}
	}
}
