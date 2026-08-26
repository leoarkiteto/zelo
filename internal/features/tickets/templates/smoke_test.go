package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/leoarkiteto/zelo/internal/features/tickets/core/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	sharedtemplates "github.com/leoarkiteto/zelo/internal/shared/templates"
)

func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func shellFixture() *sharedtemplates.ShellData {
	return &sharedtemplates.ShellData{
		User: &sharedtemplates.UserView{Email: "owner@example.com", Roles: []string{"owner"}, Initial: "O"},
		Nav:  []sharedtemplates.NavItem{{Label: "Chamados", Path: "/tickets", Active: true}},
		CSRF: "tok",
	}
}

const ptBR = i18n.LanguagePTBR

func ticketFixture() TicketView {
	return TicketView{
		ID:            "t1",
		Title:         "Vazamento na cozinha",
		CategoryLabel: "Solicitação de reparo",
		Status:        domain.StatusOpen,
		StatusLabel:   "Aberto",
		Description:   "Vazamento embaixo da pia da cozinha",
		AuthorID:      "u-syndic",
		CreatedAt:     "25 de agosto de 2026",
		UnitCode:      "A-101",
		Replies: []ReplyView{
			{Content: "Uma visita está agendada para sexta.", AuthorID: "u-syndic", CreatedAt: "26 de agosto de 2026"},
		},
	}
}

// TestTicketsPagesRender guards every tickets page against render panics and
// checks that core structure is present (rendered in pt-br).
func TestTicketsPagesRender(t *testing.T) {
	cases := []struct {
		name    string
		page    templ.Component
		want    []string
		notWant []string
	}{
		{"resident-list",
			TicketsPage(TicketsPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok",
				Tickets: []TicketView{ticketFixture()}, Flash: "Chamado criado."}),
			[]string{"Chamados", "Vazamento na cozinha", "Solicitação de reparo", "Aberto", "Chamado criado."}, nil},
		{"resident-list-empty",
			TicketsPage(TicketsPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok"}),
			[]string{"Nenhum chamado", "Abrir chamado"}, nil},
		{"new-form",
			TicketsNewPage(TicketsNewPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok",
				Categories: CategoryOptions(ptBR)}),
			[]string{"Abrir chamado", "Título", "Solicitação de reparo", "Reclamação de barulho", "Sugestão de pauta para a próxima assembleia", "Outro"}, nil},
		{"new-form-error",
			TicketsNewPage(TicketsNewPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok",
				Categories: CategoryOptions(ptBR), Error: "A descrição é obrigatória (1–2000 caracteres)."}),
			[]string{"A descrição é obrigatória"}, nil},
		{"detail-syndic-open",
			TicketDetailPage(TicketDetailPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok",
				Ticket: ticketFixture(), IsSyndic: true}),
			[]string{"Vazamento na cozinha", "Solicitação de reparo", "Vazamento embaixo da pia da cozinha", "Enviar resposta", "Encerrar chamado"}, nil},
		{"detail-closed-readonly",
			TicketDetailPage(TicketDetailPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok",
				Ticket: func() TicketView {
					v := ticketFixture()
					v.Status = domain.StatusClosed
					v.StatusLabel = "Encerrado"
					v.ClosedAt = "27 de agosto de 2026"
					return v
				}(), IsSyndic: true}),
			[]string{"Encerrado", "Encerrado em", "Uma visita está agendada"},
			[]string{"Enviar resposta"}},
		{"detail-member-readonly",
			TicketDetailPage(TicketDetailPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok",
				Ticket: ticketFixture(), IsSyndic: false}),
			[]string{"Vazamento embaixo da pia da cozinha", "Uma visita está agendada"},
			[]string{"Enviar resposta", "Encerrar chamado"}},
		{"inbox",
			TicketsInboxPage(TicketsInboxPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok",
				Tickets: []TicketView{ticketFixture()}}),
			[]string{"Caixa de chamados", "A-101", "u-syndic"}, nil},
		{"inbox-empty",
			TicketsInboxPage(TicketsInboxPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok"}),
			[]string{"Caixa vazia"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			html := renderToString(t, c.page)
			for _, want := range c.want {
				if !strings.Contains(html, want) {
					t.Errorf("rendered %s is missing %q", c.name, want)
				}
			}
			for _, absent := range c.notWant {
				if strings.Contains(html, absent) {
					t.Errorf("rendered %s must not contain %q", c.name, absent)
				}
			}
		})
	}
}
