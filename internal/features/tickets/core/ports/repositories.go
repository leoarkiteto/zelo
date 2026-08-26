// Package ports defines the persistence and collaborator interfaces the
// tickets core depends on (hexagonal architecture, constitution II).
package ports

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/tickets/core/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
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
