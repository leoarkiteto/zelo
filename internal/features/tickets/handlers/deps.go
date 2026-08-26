// Package handlers contains tickets HTTP handlers and route registration.
package handlers

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/tickets/core/domain"
	"github.com/leoarkiteto/zelo/internal/features/tickets/core/services"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// Roles returns the active roles for a user in a condominium.
type Roles interface {
	ActiveRolesForUser(ctx context.Context, userID, condominiumID string) ([]model.Role, error)
}

// AuditRecorder records security-relevant events.
type AuditRecorder interface {
	RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
}

// Units resolves a user's active unit and lists a condominium's units. The
// concrete *store.UnitStore satisfies this interface.
type Units interface {
	GetActiveUnitForUser(ctx context.Context, userID, condominiumID string) (model.Unit, error)
	ListUnitsForCondominium(ctx context.Context, condominiumID string) ([]model.Unit, error)
}

// Tickets is the use-case surface the tickets handlers depend on.
type Tickets interface {
	CreateTicket(ctx context.Context, actorID, condominiumID string, in services.TicketInput) (domain.Ticket, error)
	ReplyToTicket(ctx context.Context, actorID, ticketID, condominiumID, content string) error
	CloseTicket(ctx context.Context, actorID, ticketID, condominiumID string) error
	ListMine(ctx context.Context, userID, condominiumID string) ([]domain.Ticket, error)
	ListInbox(ctx context.Context, condominiumID string, status domain.TicketStatus) ([]domain.Ticket, error)
	GetTicket(ctx context.Context, ticketID string) (domain.Ticket, error)
	ListReplies(ctx context.Context, ticketID string) ([]domain.TicketReply, error)
}

// Deps are the collaborators used by tickets handlers.
type Deps struct {
	Roles   Roles
	Audit   AuditRecorder
	Units   Units
	Tickets Tickets
}

// Handler bundles dependencies for tickets handler methods.
type Handler struct {
	deps Deps
}
