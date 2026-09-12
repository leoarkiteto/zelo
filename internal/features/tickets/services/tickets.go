// Package services implements the ticket use cases: creation, inbox/reply,
// closing, and resident tracking.
package services

import (
	"context"
	"errors"
	"strings"

	"github.com/leoarkiteto/zelo/internal/features/tickets/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// Service errors surfaced to handlers.
var (
	// ErrInvalidInput covers malformed create payloads.
	ErrInvalidInput = errors.New("invalid ticket input")
	// ErrInvalidCategory aliases the domain category error.
	ErrInvalidCategory = domain.ErrInvalidCategory
	// ErrInvalidTitle aliases the domain title error.
	ErrInvalidTitle = domain.ErrInvalidTitle
	// ErrInvalidDescription aliases the domain description error.
	ErrInvalidDescription = domain.ErrInvalidDescription
	// ErrInvalidContent aliases the domain reply content error.
	ErrInvalidContent = domain.ErrInvalidContent
	// ErrClosed aliases the domain closed-ticket guard (FR-008).
	ErrClosed = domain.ErrClosed
	// ErrNoUnit is returned when a resident has no active unit.
	ErrNoUnit = errors.New("resident has no active unit")
	// ErrNotFound aliases the shared not-found sentinel.
	ErrNotFound = model.ErrNotFound
)

// TicketInput is a validated create payload. Title and description are the
// raw form strings; validation happens in the service.
type TicketInput struct {
	Title       string
	Category    domain.TicketCategory
	Description string
}

// TicketService implements the ticket use cases.
type TicketService struct {
	Tickets TicketStore
	Units   UnitResolver
	Audit   AuditRecorder
}

// CreateTicket validates and stores a new ticket (FR-001..FR-003). The unit is
// captured from the author's active unit; users without one get ErrNoUnit.
func (s *TicketService) CreateTicket(ctx context.Context, actorID, condominiumID string, in TicketInput) (domain.Ticket, error) {
	if !in.Category.Valid() {
		return domain.Ticket{}, ErrInvalidCategory
	}
	if err := domain.ValidateTitle(in.Title); err != nil {
		return domain.Ticket{}, ErrInvalidTitle
	}
	if err := domain.ValidateDescription(in.Description); err != nil {
		return domain.Ticket{}, ErrInvalidDescription
	}
	unit, err := s.Units.GetActiveUnitForUser(ctx, actorID, condominiumID)
	if err != nil {
		return domain.Ticket{}, ErrNoUnit
	}
	t := domain.Ticket{
		CondominiumID: condominiumID,
		AuthorID:      actorID,
		UnitID:        unit.ID,
		Title:         strings.TrimSpace(in.Title),
		Category:      in.Category,
		Description:   strings.TrimSpace(in.Description),
		Status:        domain.StatusOpen,
	}
	id, err := s.Tickets.Create(ctx, t)
	if err != nil {
		return domain.Ticket{}, err
	}
	t.ID = id
	uid := actorID
	if err := s.Audit.RecordEvent(ctx, &uid, model.AuditTicketCreated,
		map[string]any{"ticket_id": id, "category": string(t.Category), "condominium_id": condominiumID}); err != nil {
		return domain.Ticket{}, err
	}
	return t, nil
}

// ReplyToTicket attaches a syndic reply to an open ticket (FR-006, FR-008).
func (s *TicketService) ReplyToTicket(ctx context.Context, actorID, ticketID, condominiumID, content string) error {
	existing, err := s.Tickets.GetByID(ctx, ticketID)
	if err != nil {
		return err
	}
	if existing.CondominiumID != condominiumID {
		return ErrNotFound
	}
	if !existing.CanReply() {
		return ErrClosed
	}
	if err := domain.ValidateContent(content); err != nil {
		return ErrInvalidContent
	}
	if _, err := s.Tickets.AddReply(ctx, domain.TicketReply{
		TicketID: ticketID,
		AuthorID: actorID,
		Content:  strings.TrimSpace(content),
	}); err != nil {
		return err
	}
	uid := actorID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditTicketReplied,
		map[string]any{"ticket_id": ticketID, "condominium_id": condominiumID})
}

// CloseTicket closes an open ticket, recording the closing actor and date
// (FR-007, FR-008). Closing without a prior reply is allowed.
func (s *TicketService) CloseTicket(ctx context.Context, actorID, ticketID, condominiumID string) error {
	existing, err := s.Tickets.GetByID(ctx, ticketID)
	if err != nil {
		return err
	}
	if existing.CondominiumID != condominiumID {
		return ErrNotFound
	}
	if !existing.CanClose() {
		return ErrClosed
	}
	if err := s.Tickets.Close(ctx, ticketID, condominiumID, actorID); err != nil {
		return err
	}
	uid := actorID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditTicketClosed,
		map[string]any{"ticket_id": ticketID, "condominium_id": condominiumID})
}

// ListMine returns the current user's own tickets, newest first (FR-009).
func (s *TicketService) ListMine(ctx context.Context, userID, condominiumID string) ([]domain.Ticket, error) {
	return s.Tickets.ListMine(ctx, condominiumID, userID)
}

// ListInbox returns a condominium's tickets, optionally filtered by status
// (FR-005).
func (s *TicketService) ListInbox(ctx context.Context, condominiumID string, status domain.TicketStatus) ([]domain.Ticket, error) {
	return s.Tickets.ListInbox(ctx, condominiumID, status)
}

// GetTicket returns a ticket by id. Ownership is enforced by handlers.
func (s *TicketService) GetTicket(ctx context.Context, ticketID string) (domain.Ticket, error) {
	return s.Tickets.GetByID(ctx, ticketID)
}

// ListReplies returns a ticket's replies in chronological order.
func (s *TicketService) ListReplies(ctx context.Context, ticketID string) ([]domain.TicketReply, error) {
	return s.Tickets.ListReplies(ctx, ticketID)
}
