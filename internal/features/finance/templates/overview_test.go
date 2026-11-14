package templates

import (
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func overviewFixture(period domain.Period, months []services.MonthPoint) services.Overview {
	ov := services.Overview{Period: period, Months: months, Years: []int{2026, 2025}}
	for _, m := range months {
		ov.TotalInCents += m.InCents
		ov.TotalOutCents += m.OutCents
	}
	ov.BalanceCents = ov.TotalInCents - ov.TotalOutCents
	ov.Empty = ov.TotalInCents == 0 && ov.TotalOutCents == 0
	ov.Current = services.CurrentPosition{ReceivableCents: 120000, PayableCents: 80000, OverdueCents: 45000}
	return ov
}

func monthsFixture(vals [][2]int64) []services.MonthPoint {
	var out []services.MonthPoint
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, v := range vals {
		out = append(out, services.MonthPoint{
			Month: start.AddDate(0, i, 0), InCents: v[0], OutCents: v[1], BalanceCents: v[0] - v[1],
		})
	}
	return out
}

func last12() domain.Period { return domain.ParsePeriod("", time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)) }

func mustContain(t *testing.T, html string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(html, s) {
			t.Errorf("output missing %q", s)
		}
	}
}

func mustNotContain(t *testing.T, html string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if strings.Contains(html, s) {
			t.Errorf("output must not contain %q", s)
		}
	}
}

func TestOverviewPanelPopulated(t *testing.T) {
	ov := overviewFixture(last12(), monthsFixture([][2]int64{{100000, 40000}, {50000, 60000}}))
	ov.InByCategory = []services.CategoryShare{{Category: domain.CategoryCondoFee, Cents: 150000, Percent: 100}}
	ov.OutByCategory = []services.CategoryShare{
		{Category: domain.CategoryMaintenance, Cents: 60000, Percent: 60},
		{Category: domain.CategoryCleaning, Cents: 40000, Percent: 40},
	}
	html := renderToString(t, OverviewPanel(OverviewData{Locale: i18n.LanguagePTBR, Overview: ov}))

	mustContain(t, html,
		`<section id="finance-overview"`,
		`hx-get="/finance/overview"`, `hx-target="#finance-overview"`, `hx-swap="outerHTML"`,
		`method="GET"`, `action="/finance/overview"`, `name="period"`,
		"Últimos 12 meses", "Entradas", "Saídas", "Saldo do período", "A receber", "A pagar", "Em atraso",
		"R$ 1.500,00", "R$ 1.000,00", "R$ 450,00", // totals and overdue
		"bg-green-50", "bg-red-50", "bg-steel-50", "bg-amber-50",
		"fill-green-500", "fill-red-500", "stroke-steel-500",
		`role="img"`, `aria-labelledby="fo-bars-title"`, `aria-labelledby="fo-balance-title"`,
		"Entradas Jan/26: R$ 1.000,00", "Saídas Fev/26: R$ 600,00", "Fev/26: -R$ 100,00",
		"<details", "<table", "Saldo do período mês a mês", "overflow-x-auto",
		"Saídas por categoria", "Entradas por categoria", "60.0%", "40.0%", "100.0%",
		"Manutenção", "Limpeza",
		"fill-red-500", // negative point
	)
	mustNotContain(t, html, "Gerenciar finanças", "Registrar conta", "Nenhuma movimentação")
	if strings.Count(html, "<details") != 2 {
		t.Errorf("each chart needs a details table, got %d", strings.Count(html, "<details"))
	}
	// Every inline <svg> is a data chart and must carry the data-chart marker.
	mustContain(t, html, `data-chart="bars"`, `data-chart="balance"`, `data-chart="category-bar"`)
	if svgs, marked := strings.Count(html, "<svg"), strings.Count(html, "<svg data-chart="); svgs == 0 || svgs != marked {
		t.Errorf("every <svg> must carry data-chart: %d svgs, %d marked", svgs, marked)
	}
	// Order: period KPIs, current KPIs, bars, balance, categories.
	order := []string{"Previsto do mês", "Entradas × Saídas por mês", "Saldo do período mês a mês", "Saídas por categoria"}
	prev := strings.Index(html, "Saldo do período")
	for _, s := range order {
		i := strings.Index(html, s)
		if i < prev {
			t.Errorf("%q is out of order", s)
		}
		prev = i
	}
}

func TestOverviewPanelNegativeBalance(t *testing.T) {
	ov := overviewFixture(last12(), monthsFixture([][2]int64{{10000, 50000}}))
	html := renderToString(t, OverviewPanel(OverviewData{Locale: i18n.LanguagePTBR, Overview: ov}))
	mustContain(t, html, "-R$ 400,00", "text-red-700", "fill-red-500")
}

func TestOverviewPanelEmptyState(t *testing.T) {
	ov := overviewFixture(last12(), monthsFixture([][2]int64{{0, 0}, {0, 0}}))
	html := renderToString(t, OverviewPanel(OverviewData{Locale: i18n.LanguagePTBR, Overview: ov}))
	mustContain(t, html, "Nenhuma movimentação financeira nos últimos 12 meses", "R$ 0,00", "R$ 450,00")
	mustNotContain(t, html, "em nos últimos")
	htmlEN := renderToString(t, OverviewPanel(OverviewData{Locale: i18n.LanguageEN, Overview: ov}))
	mustContain(t, htmlEN, "No financial activity in the last 12 months")
	mustNotContain(t, html, "<svg", "<details", "Registrar conta", "Gerenciar finanças")

	year := domain.ParsePeriod("2025", time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC))
	ov = overviewFixture(year, nil)
	html = renderToString(t, OverviewPanel(OverviewData{Locale: i18n.LanguagePTBR, Overview: ov, IsSyndic: true}))
	mustContain(t, html, "Nenhuma movimentação financeira em 2025", `href="/finance/new"`, "Registrar conta", `href="/finance"`, "Gerenciar finanças")
	htmlEN = renderToString(t, OverviewPanel(OverviewData{Locale: i18n.LanguageEN, Overview: ov}))
	mustContain(t, htmlEN, "No financial activity in 2025")
}

func TestOverviewPanelSyndicLinks(t *testing.T) {
	ov := overviewFixture(last12(), monthsFixture([][2]int64{{100, 0}}))
	d := OverviewData{Locale: i18n.LanguagePTBR, Overview: ov, IsSyndic: true}
	html := renderToString(t, OverviewPanel(d))
	mustContain(t, html, `href="/finance"`, "Gerenciar finanças")
	mustNotContain(t, html, "Registrar conta") // only in the empty state

	d.IsSyndic = false
	mustNotContain(t, renderToString(t, OverviewPanel(d)), "Gerenciar finanças", `href="/finance"`)
}

func TestOverviewPanelPeriodSelector(t *testing.T) {
	// Selected year 2024 is not in Years but must still be selected.
	p := domain.ParsePeriod("2024", time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC))
	ov := overviewFixture(p, monthsFixture([][2]int64{{100, 0}}))
	html := renderToString(t, OverviewPanel(OverviewData{Locale: i18n.LanguagePTBR, Overview: ov}))
	mustContain(t, html, `<option value="2026">`, `<option value="2025">`, `<option value="2024" selected>`, `<option value="">`)
	mustNotContain(t, html, `<option value="" selected>`)

	ov = overviewFixture(last12(), monthsFixture([][2]int64{{100, 0}}))
	html = renderToString(t, OverviewPanel(OverviewData{Locale: i18n.LanguagePTBR, Overview: ov}))
	mustContain(t, html, `<option value="" selected>`, `hx-trigger="change, submit"`, `method="GET"`, `action="/finance/overview"`)
}

func TestOverviewPanelEnglish(t *testing.T) {
	ov := overviewFixture(last12(), monthsFixture([][2]int64{{100000, 40000}}))
	html := renderToString(t, OverviewPanel(OverviewData{Locale: i18n.LanguageEN, Overview: ov}))
	mustContain(t, html, "Income", "Expenses", "Period balance", "To receive", "To pay", "Overdue", "R$ 1,000.00", "Jan/26")
}

func TestOverviewPrivacy(t *testing.T) {
	ov := overviewFixture(last12(), monthsFixture([][2]int64{{100000, 40000}}))
	html := renderToString(t, OverviewPanel(OverviewData{Locale: i18n.LanguagePTBR, Overview: ov, IsSyndic: true}))
	mustNotContain(t, html, "Fornecedor", "Unidade", "contas em atraso")
}

func TestOverviewPage(t *testing.T) {
	ov := overviewFixture(last12(), monthsFixture([][2]int64{{100, 50}}))
	html := renderToString(t, OverviewPage(financeShellFixture(), OverviewData{Locale: i18n.LanguagePTBR, Overview: ov}))
	mustContain(t, html, `<section id="finance-overview"`, "Painel financeiro", "<h1")
}

func TestOverviewError(t *testing.T) {
	html := renderToString(t, OverviewError(i18n.LanguagePTBR, "2025"))
	mustContain(t, html, `<section id="finance-overview"`, "Tentar novamente",
		`hx-get="/finance/overview?period=2025"`, `hx-target="#finance-overview"`, `hx-swap="outerHTML"`)

	html = renderToString(t, OverviewError(i18n.LanguagePTBR, ""))
	mustContain(t, html, `hx-get="/finance/overview"`)
	mustNotContain(t, html, "period=")
}

func TestOverviewNoAccess(t *testing.T) {
	html := renderToString(t, OverviewNoAccess(i18n.LanguagePTBR))
	mustContain(t, html, `<section id="finance-overview"`, "Painel financeiro indisponível")
	mustNotContain(t, html, "hx-get", "Tentar novamente")
}
