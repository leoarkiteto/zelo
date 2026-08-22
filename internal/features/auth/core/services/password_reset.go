package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/auth/core/ports"
)

var (
	// ErrInvalidResetToken is returned for unknown or expired reset tokens.
	ErrInvalidResetToken = errors.New("invalid or expired reset token")
)

type resetEntry struct {
	userID    string
	expiresAt time.Time
}

// PasswordResetService issues single-use, 1-hour password reset tokens.
// Tokens are kept in memory for the initial single-process deployment.
type PasswordResetService struct {
	Users     ports.UserStore
	Passwords ports.PasswordHasher
	Tokens    ports.TokenHasher
	Now       func() time.Time
	Duration  time.Duration

	mu     sync.Mutex
	tokens map[string]resetEntry
}

// RequestReset generates a reset token for the account. It never reveals
// whether the email exists (spec FR-011).
func (s *PasswordResetService) RequestReset(ctx context.Context, email string) (string, error) {
	if s.tokens == nil {
		s.tokens = map[string]resetEntry{}
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Duration == 0 {
		s.Duration = time.Hour
	}
	u, err := s.Users.GetUserByEmail(ctx, email)
	if err != nil {
		return "", nil // do not reveal existence
	}
	token, err := randomResetToken()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.tokens[s.Tokens.HashToken(token)] = resetEntry{userID: u.ID, expiresAt: s.Now().Add(s.Duration)}
	s.mu.Unlock()
	return token, nil
}

// Reset validates the token and updates the password.
func (s *PasswordResetService) Reset(ctx context.Context, token, newPassword string) error {
	if err := s.Passwords.ValidatePassword(newPassword); err != nil {
		return err
	}
	hash, err := s.Passwords.Hash(newPassword)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tokens[s.Tokens.HashToken(token)]
	if !ok || (s.Now != nil && !entry.expiresAt.After(s.Now())) {
		return ErrInvalidResetToken
	}
	if err := s.Users.UpdatePassword(ctx, entry.userID, hash); err != nil {
		return err
	}
	delete(s.tokens, s.Tokens.HashToken(token))
	return nil
}

func randomResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
