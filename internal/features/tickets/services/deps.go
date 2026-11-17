// Package services defines the ticket use cases together with the narrow
// persistence and collaborator interfaces they consume (vertical slice; the
// former hexagonal core/ports layer was removed).
package services

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/tickets/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/pagination"
)

// TicketStore is the persistence port for tickets and their replies.
type TicketStore interface {
	// Create inserts a ticket and returns its id.
	Create(ctx context.Context, t domain.Ticket) (string, error)
	// GetByID returns a ticket by id.
	GetByID(ctx context.Context, id string) (domain.Ticket, error)
	// ListMine returns a user's own tickets, newest first (FR-009).
	ListMine(ctx context.Context, condominiumID, authorID string) ([]domain.Ticket, error)
	// ListInbox returns a condominium's tickets, optionally filtered by status.
	// Open tickets are ordered oldest first (FIFO); closed/all newest first.
	// An empty status returns every ticket.
	ListInbox(ctx context.Context, condominiumID string, status domain.TicketStatus) ([]domain.Ticket, error)
	// ListMinePage returns one page of a user's own tickets, newest first.
	ListMinePage(ctx context.Context, condominiumID, authorID string, requested int) ([]domain.Ticket, pagination.Page, error)
	// ListInboxPage returns one page of the inbox, optionally filtered by status.
	ListInboxPage(ctx context.Context, condominiumID string, status domain.TicketStatus, requested int) ([]domain.Ticket, pagination.Page, error)
	// AddReply appends a reply to an open ticket and returns its id.
	AddReply(ctx context.Context, r domain.TicketReply) (string, error)
	// ListReplies returns a ticket's replies in chronological order.
	ListReplies(ctx context.Context, ticketID string) ([]domain.TicketReply, error)
	// Close transitions an open ticket to closed, recording the actor.
	Close(ctx context.Context, id, condominiumID, closedBy string) error
}

// UnitResolver finds a user's active unit in a condominium. The concrete
// *store.UnitStore satisfies this interface.
type UnitResolver interface {
	GetActiveUnitForUser(ctx context.Context, userID, condominiumID string) (model.Unit, error)
}

// AuditRecorder records security-relevant events.
type AuditRecorder interface {
	RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
}
