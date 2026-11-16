package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	organisms "github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
)

func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func financeShellFixture() *organisms.ShellData {
	return &organisms.ShellData{
		User: &organisms.UserView{Email: "syndic@example.com", Roles: []string{"syndic"}, Initial: "S"},
		Nav:  []organisms.NavItem{{Label: "Financeiro", Path: "/finance", Active: true}},
		CSRF: "tok",
	}
}

func accountFixture() AccountView {
	return AccountView{
		ID:              "a1",
		Type:            domain.AccountTypePayable,
		Title:           "Manutenção do portão",
		CategoryLabel:   "Manutenção",
		Amount:          "R$ 350,00",
		DueDate:         "20/08/2026",
		EffectiveStatus: domain.StatusOverdue,
		StatusLabel:     "Em atraso",
		HasReceipt:      true,
	}
}

const ptBR = i18n.LanguagePTBR

// TestFinancePagesRender guards every finance page against render panics and
// checks that core structure is present (rendered in pt-br).
func TestFinancePagesRender(t *testing.T) {
	cases := []struct {
		name string
		page templ.Component
		want []string
	}{
		{"finance-dashboard",
			FinancePage(FinancePageData{
				Shell:      financeShellFixture(),
				Locale:     ptBR,
				CSRF:       "tok",
				Accounts:   []AccountView{accountFixture()},
				Categories: CategoryOptions(ptBR, domain.PayableCategories()),
				Statuses:   StatusOptions(ptBR),
				Types:      TypeOptions(ptBR),
				Summary:    SummaryView{TotalReceivable: "R$ 500,00", TotalPayable: "R$ 350,00", ProjectedBalance: "R$ 150,00", RealizedBalance: "R$ 0,00"},
				Flash:      "Conta criada.",
			}),
			[]string{"Contas a pagar e receber", "Manutenção do portão", "R$ 350,00", "Em atraso", "Conta criada."}},
		{"finance-empty",
			FinancePage(FinancePageData{Shell: financeShellFixture(), Locale: ptBR, CSRF: "tok"}),
			[]string{"Nenhuma conta encontrada"}},
		{"finance-form-payable",
			FinanceFormPage(FinanceFormData{Shell: financeShellFixture(), Locale: ptBR, CSRF: "tok",
				AccountType: domain.AccountTypePayable,
				Values:      FormValues{Type: "payable"},
				Categories:  CategoryOptions(ptBR, domain.PayableCategories())}),
			[]string{"Cadastrar despesa", "Fornecedor / credor", "Comprovante de pagamento"}},
		{"finance-form-receivable",
			FinanceFormPage(FinanceFormData{Shell: financeShellFixture(), Locale: ptBR, CSRF: "tok",
				AccountType: domain.AccountTypeReceivable,
				Values:      FormValues{Type: "receivable"},
				Categories:  CategoryOptions(ptBR, domain.ReceivableCategories()),
				Units:       []UnitOption{{ID: "u1", Code: "A-101"}}}),
			[]string{"Lançar receita", "Unidade responsável", "A-101"}},
		{"settle",
			SettlePage(SettleData{Shell: financeShellFixture(), Locale: ptBR, CSRF: "tok", Account: accountFixture()}),
			[]string{"Liquidar", "Data da transação", "Manutenção do portão"}},
		{"cancel",
			CancelPage(CancelData{Shell: financeShellFixture(), Locale: ptBR, CSRF: "tok", Account: accountFixture()}),
			[]string{"Cancelar conta", "Isso marca a conta como cancelada."}},
		{"statement",
			StatementPage(StatementPageData{Shell: financeShellFixture(), Locale: ptBR,
				UnitID: "un1", UnitCode: "101",
				Units:   []UnitOption{{ID: "un1", Code: "101"}, {ID: "un2", Code: "102"}},
				Pending: []StatementRowView{{ID: "a1", Title: "Quota agosto", Amount: "R$ 350.00", Overdue: true, StatusLabel: "Vencida"}},
				Settled: []StatementRowView{{ID: "a2", Title: "Quota julho", Amount: "R$ 350.00", SettlementDate: "05/07/2026", CanReceipt: true}}}),
			[]string{"Conta corrente", "Quota agosto", "bg-red-50", `name="unit"`, "/finance/a2/recibo"}},
		{"statement-no-unit",
			StatementPage(StatementPageData{Shell: financeShellFixture(), Locale: ptBR, NoUnit: true}),
			[]string{"unidade ativa"}},
		{"receipt",
			ReceiptPage(ReceiptPageData{Shell: financeShellFixture(), Locale: ptBR, Number: "AB12CD34",
				Condominium: "Edifício Sol", UnitCode: "101", Title: "Quota agosto", PeriodLabel: "08/2026",
				Base: "R$ 300.00", Reserve: "R$ 50.00", Amount: "R$ 350.00", SettledOn: "05/08/2026", BackHref: "/finance/my-charges"}),
			[]string{"Recibo", "AB12CD34", "Edifício Sol", "08/2026", "R$ 350.00", "05/08/2026", "receipt-sheet", "no-print"}},
		{"quotas-card-resident",
			QuotasCard(QuotasCardData{Locale: ptBR, Resident: &ResidentCardView{HasNext: true, NextAmount: "R$ 350.00", NextDue: "10/12/2026"}}),
			[]string{`id="quotas-card"`, "Próxima taxa", "R$ 350.00"}},
		{"quotas-card-syndic",
			QuotasCard(QuotasCardData{Locale: ptBR, Syndic: &SyndicCardView{HasBudget: true, PeriodLabel: "12/2026", PreviewHref: "/finance/budgets/b1/issuances/12/preview"}}),
			[]string{"Não emitida", "/finance/budgets/b1/issuances/12/preview"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			html := renderToString(t, c.page)
			for _, want := range c.want {
				if !strings.Contains(html, want) {
					t.Errorf("rendered %s is missing %q", c.name, want)
				}
			}
		})
	}
}

func TestMoneyFormatting(t *testing.T) {
	cases := []struct {
		lang  i18n.Language
		cents int64
		want  string
	}{
		{i18n.LanguagePTBR, 123456, "R$ 1.234,56"},
		{i18n.LanguageEN, 123456, "R$ 1,234.56"},
		{i18n.LanguagePTBR, 50000, "R$ 500,00"},
		{i18n.LanguageEN, 50000, "R$ 500.00"},
		{i18n.LanguagePTBR, 0, "R$ 0,00"},
	}
	for _, c := range cases {
		if got := Money(context.Background(), c.lang, c.cents); got != c.want {
			t.Errorf("Money(%v, %d) = %q, want %q", c.lang, c.cents, got, c.want)
		}
	}
}

func TestMoneyFormattingEUR(t *testing.T) {
	ctx := money.WithCurrency(context.Background(), money.EUR)
	if got, want := Money(ctx, i18n.LanguagePTPT, 123456), "1 234,56 €"; got != want {
		t.Errorf("Money EUR pt-pt = %q, want %q", got, want)
	}
	if got, want := compactMoney(ctx, i18n.LanguagePTPT, 150000), "1,5k €"; got != want {
		t.Errorf("compactMoney EUR pt-pt = %q, want %q", got, want)
	}
	if got, want := compactMoney(ctx, i18n.LanguageEN, 150000), "€1.5k"; got != want {
		t.Errorf("compactMoney EUR en = %q, want %q", got, want)
	}
}

func TestOverviewRendersEUR(t *testing.T) {
	ctx := money.WithCurrency(context.Background(), money.EUR)
	var buf bytes.Buffer
	ov := overviewFixture(last12(), monthsFixture([][2]int64{{123456, 40000}}))
	d := OverviewData{Locale: i18n.LanguagePTPT, Overview: ov}
	if err := OverviewPanel(d).Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if !strings.Contains(html, "1 234,56 €") {
		t.Errorf("EUR amount missing from overview")
	}
	if strings.Contains(html, "R$") {
		t.Errorf("unexpected BRL symbol in EUR overview")
	}
}
