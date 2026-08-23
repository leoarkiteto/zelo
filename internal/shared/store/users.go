package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// UserStore persists users.
type UserStore struct {
	db *sql.DB
}

// NewUserStore creates a UserStore.
func NewUserStore(db *sql.DB) *UserStore { return &UserStore{db: db} }

// CreateUser inserts a user.
func (s *UserStore) CreateUser(ctx context.Context, u model.User) (string, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, status)
		VALUES (lower($1), $2, $3)
		RETURNING id, created_at, updated_at`, u.Email, u.PasswordHash, u.Status)
	var id string
	if err := row.Scan(&id, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if IsUniqueViolation(err) {
			return "", ErrDuplicate
		}
		return "", fmt.Errorf("create user: %w", err)
	}
	return id, nil
}

// GetUserByEmail returns a user by normalized email.
func (s *UserStore) GetUserByEmail(ctx context.Context, email string) (model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, status, failed_sign_in_count, locked_until, created_at, updated_at,
		       COALESCE(language_preference, '')
		FROM users WHERE lower(email) = lower($1)`, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Status, &u.FailedSignInCount, &u.LockedUntil,
		&u.CreatedAt, &u.UpdatedAt, &u.LanguagePreference)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// GetUserByID returns a user by id.
func (s *UserStore) GetUserByID(ctx context.Context, id string) (model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, status, failed_sign_in_count, locked_until, created_at, updated_at,
		       COALESCE(language_preference, '')
		FROM users WHERE id = $1`, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Status, &u.FailedSignInCount, &u.LockedUntil,
		&u.CreatedAt, &u.UpdatedAt, &u.LanguagePreference)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// UpdateLanguagePreference saves the user's interface language preference.
// The caller must pass a valid language code ('en' or 'pt-br').
func (s *UserStore) UpdateLanguagePreference(ctx context.Context, userID, language string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET language_preference = $1, updated_at = now() WHERE id = $2`,
		language, userID)
	if err != nil {
		return fmt.Errorf("update language preference: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePassword replaces the password hash.
func (s *UserStore) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`,
		passwordHash, userID)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordFailedSignIn increments the failed sign-in counter.
func (s *UserStore) RecordFailedSignIn(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET failed_sign_in_count = failed_sign_in_count + 1, updated_at = now() WHERE id = $1`,
		userID)
	return err
}

// ApplyLock sets a lock and resets the failure counter.
func (s *UserStore) ApplyLock(ctx context.Context, userID string, until time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET locked_until = $1, failed_sign_in_count = 0, updated_at = now() WHERE id = $2`,
		until, userID)
	return err
}

// ClearLock removes an account lock and resets the failure counter.
func (s *UserStore) ClearLock(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET locked_until = NULL, failed_sign_in_count = 0, updated_at = now() WHERE id = $1`,
		userID)
	return err
}

func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}
