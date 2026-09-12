package services

import (
	"context"
	"errors"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

var (
	// ErrInvalidCredentials is returned when email/password do not match.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrAccountLocked is returned when the account is temporarily locked.
	ErrAccountLocked = errors.New("account temporarily locked")
)

// AuthService authenticates users and enforces the sign-in lockout (US2).
// Collaborators are concrete function values wired in cmd/web/main.go; tests
// pass method values or closures instead of interface-based fakes.
type AuthService struct {
	GetUserByEmail     func(ctx context.Context, email string) (model.User, error)
	RecordFailedSignIn func(ctx context.Context, userID string) error
	ApplyLock          func(ctx context.Context, userID string, until time.Time) error
	ClearLock          func(ctx context.Context, userID string) error
	FirstActiveRole    func(ctx context.Context, userID string) (model.RoleAssignment, error)
	VerifyPassword     func(hash, password string) (bool, error)
	RecordEvent        func(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
	LockThreshold      int
	LockDuration       time.Duration
	Now                func() time.Time
}

// Authenticate verifies credentials and returns the user and their current
// condominium id (empty when the user has no active roles).
func (s *AuthService) Authenticate(ctx context.Context, email, password string) (model.User, string, error) {
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.LockThreshold == 0 {
		s.LockThreshold = 5
	}
	if s.LockDuration == 0 {
		s.LockDuration = 15 * time.Minute
	}

	u, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		return model.User{}, "", ErrInvalidCredentials
	}
	if u.Status != model.UserStatusActive {
		return model.User{}, "", ErrInvalidCredentials
	}
	now := s.Now()
	if u.LockedUntil != nil && u.LockedUntil.After(now) {
		return model.User{}, "", ErrAccountLocked
	}

	ok, err := s.VerifyPassword(u.PasswordHash, password)
	if err != nil {
		return model.User{}, "", ErrInvalidCredentials
	}
	if !ok {
		uid := u.ID
		_ = s.RecordFailedSignIn(ctx, u.ID)
		_ = s.RecordEvent(ctx, &uid, model.AuditFailedSignIn, map[string]any{"email": email})
		u.FailedSignInCount++
		if u.FailedSignInCount >= s.LockThreshold {
			until := now.Add(s.LockDuration)
			_ = s.ApplyLock(ctx, u.ID, until)
			_ = s.RecordEvent(ctx, &uid, model.AuditAccountLocked,
				map[string]any{"until": until.UTC().Format(time.RFC3339)})
			return model.User{}, "", ErrAccountLocked
		}
		return model.User{}, "", ErrInvalidCredentials
	}

	_ = s.ClearLock(ctx, u.ID)
	condominiumID := ""
	if ra, err := s.FirstActiveRole(ctx, u.ID); err == nil {
		condominiumID = ra.CondominiumID
	}
	uid := u.ID
	_ = s.RecordEvent(ctx, &uid, model.AuditSignIn, map[string]any{"email": email})
	return u, condominiumID, nil
}
