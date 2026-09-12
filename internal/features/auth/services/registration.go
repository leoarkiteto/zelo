package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

var (
	// ErrInvalidInvitation is returned when the invitation is missing, used,
	// revoked, or expired.
	ErrInvalidInvitation = errors.New("invalid or expired invitation")
	// ErrEmailTaken is returned when the email is already registered.
	ErrEmailTaken = errors.New("email already registered")
	// ErrEmailMismatch is returned when the email does not match the invitation.
	ErrEmailMismatch = errors.New("email does not match the invitation")
	// ErrInvalidEmail is returned when the email is malformed.
	ErrInvalidEmail = errors.New("invalid email")
	// ErrDuplicate aliases the domain-level duplicate sentinel.
	ErrDuplicate = model.ErrDuplicate
)

// RegistrationService registers users via syndic-issued invitations (US1).
// Collaborators are concrete function values wired in cmd/web/main.go.
type RegistrationService struct {
	CreateUser               func(ctx context.Context, u model.User) (string, error)
	GrantRole                func(ctx context.Context, a model.RoleAssignment) error
	GetInvitationByTokenHash func(ctx context.Context, tokenHash string) (model.Invitation, error)
	MarkInvitationAccepted   func(ctx context.Context, id string) error
	HashPassword             func(password string) (string, error)
	ValidatePassword         func(password string) error
	HashToken                func(token string) string
	Now                      func() time.Time
}

// Register validates the invitation and creates the user with the invited role.
func (s *RegistrationService) Register(ctx context.Context, token, email, password string) error {
	if s.Now == nil {
		s.Now = time.Now
	}
	if err := s.ValidatePassword(password); err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") {
		return ErrInvalidEmail
	}
	hash, err := s.HashPassword(password)
	if err != nil {
		return err
	}

	inv, err := s.GetInvitationByTokenHash(ctx, s.HashToken(token))
	if err != nil {
		return ErrInvalidInvitation
	}
	if inv.Status != model.InvitationPending || !inv.ExpiresAt.After(s.Now()) {
		return ErrInvalidInvitation
	}
	if inv.InvitedEmail != "" && !strings.EqualFold(inv.InvitedEmail, email) {
		return ErrEmailMismatch
	}

	userID, err := s.CreateUser(ctx, model.User{
		Email:        email,
		PasswordHash: hash,
		Status:       model.UserStatusActive,
	})
	if err != nil {
		if errors.Is(err, ErrDuplicate) {
			return ErrEmailTaken
		}
		return err
	}
	if err := s.GrantRole(ctx, model.RoleAssignment{
		UserID:        userID,
		CondominiumID: inv.CondominiumID,
		Role:          inv.InvitedRole,
	}); err != nil {
		return err
	}
	return s.MarkInvitationAccepted(ctx, inv.ID)
}
