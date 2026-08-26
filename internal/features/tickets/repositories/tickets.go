// Package repositories implements the tickets persistence adapters using
// database/sql and plain SQL (constitution VI).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/leoarkiteto/zelo/internal/features/tickets/core/domain"
	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
)

// TicketStore persists tickets and their replies.
type TicketStore struct {
	db *sql.DB
}

// NewTicketStore creates a TicketStore.
func NewTicketStore(db *sql.DB) *TicketStore { return &TicketStore{db: db} }

// Create inserts a ticket and returns its id.
func (s *TicketStore) Create(ctx context.Context, t domain.Ticket) (string, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO tickets (condominium_id, author_id, unit_id, title, category, description, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		t.CondominiumID, t.AuthorID, sharedstore.NullString(t.UnitID), t.Title,
		t.Category, t.Description, t.Status)
	var id string
	if err := row.Scan(&id, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return "", fmt.Errorf("create ticket: %w", err)
	}
	return id, nil
}

// GetByID returns a ticket by id.
func (s *TicketStore) GetByID(ctx context.Context, id string) (domain.Ticket, error) {
	var t domain.Ticket
	var unitID, closedBy sql.NullString
	var closedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, condominium_id, author_id, unit_id, title, category, description,
			status, closed_at, closed_by, created_at, updated_at
		FROM tickets WHERE id = $1`, id).Scan(
		&t.ID, &t.CondominiumID, &t.AuthorID, &unitID, &t.Title, &t.Category,
		&t.Description, &t.Status, &closedAt, &closedBy, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Ticket{}, sharedstore.ErrNotFound
	}
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("get ticket: %w", err)
	}
	t.UnitID = unitID.String
	t.ClosedBy = closedBy.String
	if closedAt.Valid {
		t.ClosedAt = &closedAt.Time
	}
	return t, nil
}

// ListMine returns a user's own tickets, newest first (FR-009).
func (s *TicketStore) ListMine(ctx context.Context, condominiumID, authorID string) ([]domain.Ticket, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, condominium_id, author_id, unit_id, title, category, description,
			status, closed_at, closed_by, created_at, updated_at
		FROM tickets
		WHERE condominium_id = $1 AND author_id = $2
		ORDER BY created_at DESC`, condominiumID, authorID)
	if err != nil {
		return nil, fmt.Errorf("list my tickets: %w", err)
	}
	defer rows.Close()
	return scanTickets(rows)
}

// ListInbox returns a condominium's tickets, optionally filtered by status.
// Open tickets are handled oldest first (FIFO); closed and unfiltered lists
// show newest first.
func (s *TicketStore) ListInbox(ctx context.Context, condominiumID string, status domain.TicketStatus) ([]domain.Ticket, error) {
	order := "created_at DESC"
	if status == domain.StatusOpen {
		order = "created_at ASC"
	}
	query := `
		SELECT id, condominium_id, author_id, unit_id, title, category, description,
			status, closed_at, closed_by, created_at, updated_at
		FROM tickets WHERE condominium_id = $1`
	args := []any{condominiumID}
	if status != "" && status.Valid() {
		query += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, status)
	}
	query += " ORDER BY " + order

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list inbox: %w", err)
	}
	defer rows.Close()
	return scanTickets(rows)
}

// AddReply appends a reply to an open ticket. Closed or missing tickets yield
// ErrNotFound, so the closed read-only rule (FR-008) holds at the data layer.
func (s *TicketStore) AddReply(ctx context.Context, r domain.TicketReply) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO ticket_replies (ticket_id, author_id, content)
		SELECT $1, $2, $3
		WHERE EXISTS (SELECT 1 FROM tickets WHERE id = $1 AND status = 'open')
		RETURNING id`, r.TicketID, r.AuthorID, r.Content).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", sharedstore.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("add reply: %w", err)
	}
	return id, nil
}

// ListReplies returns a ticket's replies in chronological order.
func (s *TicketStore) ListReplies(ctx context.Context, ticketID string) ([]domain.TicketReply, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, ticket_id, author_id, content, created_at
		FROM ticket_replies WHERE ticket_id = $1
		ORDER BY created_at ASC`, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list replies: %w", err)
	}
	defer rows.Close()
	var out []domain.TicketReply
	for rows.Next() {
		var r domain.TicketReply
		if err := rows.Scan(&r.ID, &r.TicketID, &r.AuthorID, &r.Content, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Close transitions an open ticket to closed, recording the actor and date.
// Closed or missing tickets yield ErrNotFound.
func (s *TicketStore) Close(ctx context.Context, id, condominiumID, closedBy string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE tickets
		SET status = 'closed', closed_at = now(), closed_by = $3, updated_at = now()
		WHERE id = $1 AND condominium_id = $2 AND status = 'open'`,
		id, condominiumID, closedBy)
	if err != nil {
		return fmt.Errorf("close ticket: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sharedstore.ErrNotFound
	}
	return nil
}

func scanTickets(rows *sql.Rows) ([]domain.Ticket, error) {
	var out []domain.Ticket
	for rows.Next() {
		var t domain.Ticket
		var unitID, closedBy sql.NullString
		var closedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.CondominiumID, &t.AuthorID, &unitID, &t.Title,
			&t.Category, &t.Description, &t.Status, &closedAt, &closedBy,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.UnitID = unitID.String
		t.ClosedBy = closedBy.String
		if closedAt.Valid {
			t.ClosedAt = &closedAt.Time
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
