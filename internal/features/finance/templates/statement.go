package templates

import (
	"fmt"
	"strconv"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	organisms "github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
)

// StatementRowView is one receivable line of the Conta corrente.
type StatementRowView struct {
	ID             string
	Title          string
	PeriodLabel    string
	Amount         string
	DueDate        string
	SettlementDate string
	StatusLabel    string
	Overdue        bool
	// CanReceipt is true for settled receivables (link to the Recibo).
	CanReceipt bool
}

// StatementPageData backs the per-unit Conta corrente (FR-18..20).
type StatementPageData struct {
	Shell  *organisms.ShellData
	Locale i18n.Language
	NoUnit bool

	UnitID   string
	UnitCode string
	// Units are the selector choices; the selector shows only with more than one.
	Units []UnitOption

	Pending      []StatementRowView
	Settled      []StatementRowView
	Canceled     []StatementRowView
	PendingTotal string
	OverdueTotal string
	OverdueCount int
}

// ReceiptPageData backs the printable Recibo (FR-26).
type ReceiptPageData struct {
	Shell       *organisms.ShellData
	Locale      i18n.Language
	Number      string
	Condominium string
	UnitCode    string
	Title       string
	// PeriodLabel, Base and Reserve are set for quotas only.
	PeriodLabel string
	Base        string
	Reserve     string
	Amount      string
	SettledOn   string
	BackHref    string
}

// ResidentCardView is the morador part of the dashboard card.
type ResidentCardView struct {
	OverdueCount int
	OverdueTotal string
	HasNext      bool
	NextAmount   string
	NextDue      string
}

// SyndicCardView is the síndico part of the dashboard card.
type SyndicCardView struct {
	HasBudget bool
	Issued    bool
	// PeriodLabel is the localised current period ("dezembro de 2026").
	PeriodLabel string
	// PreviewHref opens the issuance preview of the current period.
	PreviewHref string
	Progress    string
}

// QuotasCardData backs the HX dashboard card (FR-22).
type QuotasCardData struct {
	Locale   i18n.Language
	Resident *ResidentCardView
	Syndic   *SyndicCardView
}

func itoa(n int) string { return strconv.Itoa(n) }

// SettledProgress formats "issued: X of Y settled".
func SettledProgress(locale i18n.Language, settled, total int) string {
	return fmt.Sprintf(i18n.T(locale, "quotas.card.settled_of"), settled, total)
}
