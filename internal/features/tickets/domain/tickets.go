// Package domain contains ticket-specific domain rules: categories, statuses,
// text validation, and status transitions.
package domain

import (
	"errors"
	"strings"
	"time"
)

// Domain validation errors returned by the tickets package.
var (
	// ErrInvalidCategory is returned when the category is not one of the
	// supported keys (FR-001).
	ErrInvalidCategory = errors.New("invalid category")
	// ErrInvalidTitle is returned when the title is empty or too long.
	ErrInvalidTitle = errors.New("invalid title")
	// ErrInvalidDescription is returned when the description is empty or too long (FR-002).
	ErrInvalidDescription = errors.New("invalid description")
	// ErrInvalidContent is returned when a reply is empty or too long.
	ErrInvalidContent = errors.New("invalid reply content")
	// ErrClosed is returned when replying to or closing a closed ticket (FR-008).
	ErrClosed = errors.New("ticket is closed")
)

// TicketCategory is a stable category key; display labels live in the i18n catalog.
type TicketCategory string

// Supported ticket categories (spec FR-001).
const (
	CategoryRepair         TicketCategory = "repair"
	CategoryNoiseComplaint TicketCategory = "noise_complaint"
	CategoryAssemblyTopic  TicketCategory = "assembly_topic"
	CategoryOther          TicketCategory = "other"
)

// Valid reports whether c is a supported category.
func (c TicketCategory) Valid() bool {
	return c == CategoryRepair || c == CategoryNoiseComplaint ||
		c == CategoryAssemblyTopic || c == CategoryOther
}

// Categories returns the supported category keys in display order.
func Categories() []TicketCategory {
	return []TicketCategory{CategoryRepair, CategoryNoiseComplaint, CategoryAssemblyTopic, CategoryOther}
}

// TicketStatus is the persisted lifecycle status (FR-004).
type TicketStatus string

const (
	// StatusOpen is the initial status for new tickets.
	StatusOpen TicketStatus = "open"
	// StatusClosed is terminal and read-only.
	StatusClosed TicketStatus = "closed"
)

// Valid reports whether s is a supported status.
func (s TicketStatus) Valid() bool {
	return s == StatusOpen || s == StatusClosed
}

const (
	maxTitleLen   = 120
	maxTextLen    = 2000
)

// ValidateTitle checks a ticket title: trimmed non-empty and at most 120
// characters.
func ValidateTitle(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || len([]rune(s)) > maxTitleLen {
		return ErrInvalidTitle
	}
	return nil
}

// ValidateDescription checks a ticket description: trimmed non-empty and at
// most 2000 characters (FR-002).
func ValidateDescription(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || len([]rune(s)) > maxTextLen {
		return ErrInvalidDescription
	}
	return nil
}

// ValidateContent checks a reply: trimmed non-empty and at most 2000 characters.
func ValidateContent(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || len([]rune(s)) > maxTextLen {
		return ErrInvalidContent
	}
	return nil
}

// Ticket is a persisted request, complaint, or suggestion.
type Ticket struct {
	ID            string
	CondominiumID string
	AuthorID      string
	UnitID        string
	Title         string
	Category      TicketCategory
	Description   string
	Status        TicketStatus
	ClosedAt      *time.Time
	ClosedBy      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CanReply reports whether the ticket accepts new replies (FR-006/FR-008).
func (t Ticket) CanReply() bool { return t.Status == StatusOpen }

// CanClose reports whether the ticket can be closed (FR-007/FR-008).
func (t Ticket) CanClose() bool { return t.Status == StatusOpen }

// TicketReply is a message from the syndic attached to a ticket.
type TicketReply struct {
	ID        string
	TicketID  string
	AuthorID  string
	Content   string
	CreatedAt time.Time
}
