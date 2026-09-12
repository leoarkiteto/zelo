package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
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
		{"my-charges",
			ChargesPage(ChargesPageData{Shell: financeShellFixture(), Locale: ptBR, CSRF: "tok",
				Charges: []AccountView{accountFixture()}}),
			[]string{"Minhas cobranças", "Manutenção do portão"}},
		{"my-charges-no-unit",
			ChargesPage(ChargesPageData{Shell: financeShellFixture(), Locale: ptBR, CSRF: "tok", NoUnit: true}),
			[]string{"unidade ativa"}},
		{"health",
			HealthPage(HealthPageData{Shell: financeShellFixture(), Locale: ptBR, CSRF: "tok", Period: "2026-08",
				TotalSpent: "R$ 350,00", TotalCollected: "R$ 500,00"}),
			[]string{"Saúde financeira", "R$ 350,00", "R$ 500,00"}},
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
		if got := Money(c.lang, c.cents); got != c.want {
			t.Errorf("Money(%v, %d) = %q, want %q", c.lang, c.cents, got, c.want)
		}
	}
}
