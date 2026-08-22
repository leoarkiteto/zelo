package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// InvitationStore persists registration invitations.
type InvitationStore struct {
	db *sql.DB
}

// NewInvitationStore creates an InvitationStore.
func NewInvitationStore(db *sql.DB) *InvitationStore { return &InvitationStore{db: db} }

// CreateInvitation inserts a pending invitation.
func (s *InvitationStore) CreateInvitation(ctx context.Context, inv model.Invitation) (string, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO invitations (token_hash, condominium_id, unit_id, invited_role, invited_email, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		inv.TokenHash, inv.CondominiumID, inv.UnitID, inv.InvitedRole, NullString(inv.InvitedEmail),
		inv.ExpiresAt, inv.CreatedBy)
	var id string
	if err := row.Scan(&id, &inv.CreatedAt); err != nil {
		return "", fmt.Errorf("create invitation: %w", err)
	}
	return id, nil
}

// GetInvitationByTokenHash returns an invitation by its hashed token.
func (s *InvitationStore) GetInvitationByTokenHash(ctx context.Context, tokenHash string) (model.Invitation, error) {
	var inv model.Invitation
	var email sql.NullString
	var acceptedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, token_hash, condominium_id, unit_id, invited_role, invited_email, status, expires_at, created_by, created_at, accepted_at
		FROM invitations WHERE token_hash = $1`, tokenHash).Scan(
		&inv.ID, &inv.TokenHash, &inv.CondominiumID, &inv.UnitID, &inv.InvitedRole, &email,
		&inv.Status, &inv.ExpiresAt, &inv.CreatedBy, &inv.CreatedAt, &acceptedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Invitation{}, ErrNotFound
	}
	if err != nil {
		return model.Invitation{}, fmt.Errorf("get invitation: %w", err)
	}
	inv.InvitedEmail = email.String
	if acceptedAt.Valid {
		inv.AcceptedAt = &acceptedAt.Time
	}
	return inv, nil
}

// MarkInvitationAccepted sets the invitation as accepted.
func (s *InvitationStore) MarkInvitationAccepted(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE invitations SET status = 'accepted', accepted_at = now() WHERE id = $1 AND status = 'pending'`, id)
	if err != nil {
		return fmt.Errorf("accept invitation: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeInvitation revokes a pending invitation.
func (s *InvitationStore) RevokeInvitation(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE invitations SET status = 'revoked' WHERE id = $1 AND status = 'pending'`, id)
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListInvitationsForCondominium returns invitations for a condominium.
func (s *InvitationStore) ListInvitationsForCondominium(ctx context.Context, condominiumID string) ([]model.Invitation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, token_hash, condominium_id, unit_id, invited_role, invited_email, status, expires_at, created_by, created_at, accepted_at
		FROM invitations WHERE condominium_id = $1 ORDER BY created_at DESC`, condominiumID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()
	var out []model.Invitation
	for rows.Next() {
		var inv model.Invitation
		var email sql.NullString
		var acceptedAt sql.NullTime
		if err := rows.Scan(&inv.ID, &inv.TokenHash, &inv.CondominiumID, &inv.UnitID, &inv.InvitedRole,
			&email, &inv.Status, &inv.ExpiresAt, &inv.CreatedBy, &inv.CreatedAt, &acceptedAt); err != nil {
			return nil, err
		}
		inv.InvitedEmail = email.String
		if acceptedAt.Valid {
			inv.AcceptedAt = &acceptedAt.Time
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func NullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
