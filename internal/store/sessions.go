package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/leoarkiteto/zelo/internal/model"
)

// SessionStore persists server-side sessions.
type SessionStore struct {
	db *sql.DB
}

// NewSessionStore creates a SessionStore.
func NewSessionStore(db *sql.DB) *SessionStore { return &SessionStore{db: db} }

// CreateSession inserts a session row.
func (s *SessionStore) CreateSession(ctx context.Context, sess model.Session) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, condominium_id, csrf_token, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		sess.TokenHash, sess.UserID, nullString(sess.CondominiumID), sess.CSRFToken, sess.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// GetSessionByTokenHash returns a session by hashed token.
func (s *SessionStore) GetSessionByTokenHash(ctx context.Context, tokenHash string) (model.Session, error) {
	var sess model.Session
	var condo sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT token_hash, user_id, condominium_id, csrf_token, created_at, expires_at
		FROM sessions WHERE token_hash = $1`, tokenHash).Scan(
		&sess.TokenHash, &sess.UserID, &condo, &sess.CSRFToken, &sess.CreatedAt, &sess.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Session{}, ErrNotFound
	}
	if err != nil {
		return model.Session{}, fmt.Errorf("get session: %w", err)
	}
	sess.CondominiumID = condo.String
	return sess, nil
}

// UpdateSessionExpiry extends the session lifetime (idle refresh).
func (s *SessionStore) UpdateSessionExpiry(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = $1 WHERE token_hash = $2`, expiresAt, tokenHash)
	if err != nil {
		return fmt.Errorf("update session expiry: %w", err)
	}
	return nil
}

// DeleteSession removes a session row.
func (s *SessionStore) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes expired session rows.
func (s *SessionStore) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
