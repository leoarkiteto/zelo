// Package templates contains the tickets page views (Templ) and their data
// view types.
package templates

import (
	"github.com/leoarkiteto/zelo/internal/features/tickets/core/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	sharedtemplates "github.com/leoarkiteto/zelo/internal/shared/templates"
)

// CategoryOption is a selectable category key with its localized label.
type CategoryOption struct {
	Key   string
	Label string
}

// FormValues carries the raw form input for re-rendering on validation errors.
type FormValues struct {
	Title       string
	Category    string
	Description string
}

// ReplyView is the display shape of a ticket reply.
type ReplyView struct {
	Content   string
	AuthorID  string
	CreatedAt string
}

// TicketView is the display shape of a ticket.
type TicketView struct {
	ID            string
	Title         string
	CategoryLabel string
	Status        domain.TicketStatus
	StatusLabel   string
	Description   string
	AuthorID      string
	CreatedAt     string
	ClosedAt      string
	UnitCode      string
	Replies       []ReplyView
}

// TicketsPageData backs the resident ticket list page.
type TicketsPageData struct {
	Shell   *sharedtemplates.ShellData
	Locale  i18n.Language
	CSRF    string
	Tickets []TicketView
	Flash   string
}

// TicketsNewPageData backs the ticket creation form page.
type TicketsNewPageData struct {
	Shell      *sharedtemplates.ShellData
	Locale     i18n.Language
	CSRF       string
	Values     FormValues
	Categories []CategoryOption
	Error      string
}

// TicketDetailPageData backs the ticket detail page.
type TicketDetailPageData struct {
	Shell    *sharedtemplates.ShellData
	Locale   i18n.Language
	CSRF     string
	Ticket   TicketView
	IsSyndic bool
	Flash    string
}

// TicketsInboxPageData backs the syndic inbox page.
type TicketsInboxPageData struct {
	Shell   *sharedtemplates.ShellData
	Locale  i18n.Language
	CSRF    string
	Tickets []TicketView
	Status  string
	Flash   string
}

// CategoryLabel returns the localized label for a category key.
func CategoryLabel(locale i18n.Language, c domain.TicketCategory) string {
	return i18n.T(locale, i18n.MessageKey("tickets.category."+string(c)))
}

// CategoryOptions builds the selectable categories.
func CategoryOptions(locale i18n.Language) []CategoryOption {
	keys := domain.Categories()
	out := make([]CategoryOption, 0, len(keys))
	for _, c := range keys {
		out = append(out, CategoryOption{Key: string(c), Label: CategoryLabel(locale, c)})
	}
	return out
}

// StatusLabel returns the localized label for a status.
func StatusLabel(locale i18n.Language, s domain.TicketStatus) string {
	switch s {
	case domain.StatusOpen:
		return i18n.T(locale, "tickets.status.open")
	case domain.StatusClosed:
		return i18n.T(locale, "tickets.status.closed")
	default:
		return string(s)
	}
}

// statusBadge returns the badge variant for a status.
func statusBadge(s domain.TicketStatus) string {
	switch s {
	case domain.StatusOpen:
		return "badge-info"
	default:
		return "badge-neutral"
	}
}

// FilterLinkClass styles the inbox filter tabs.
func FilterLinkClass(active bool) string {
	if active {
		return "btn btn-sm btn-primary"
	}
	return "btn btn-sm btn-secondary"
}
