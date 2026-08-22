package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/auth/core/ports"
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
type RegistrationService struct {
	Users       ports.UserStore
	Roles       ports.RoleStore
	Invitations ports.InvitationStore
	Passwords   ports.PasswordHasher
	Tokens      ports.TokenHasher
	Now         func() time.Time
}

// Register validates the invitation and creates the user with the invited role.
func (s *RegistrationService) Register(ctx context.Context, token, email, password string) error {
	if s.Now == nil {
		s.Now = time.Now
	}
	if err := s.Passwords.ValidatePassword(password); err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") {
		return ErrInvalidEmail
	}
	hash, err := s.Passwords.Hash(password)
	if err != nil {
		return err
	}

	inv, err := s.Invitations.GetInvitationByTokenHash(ctx, s.Tokens.HashToken(token))
	if err != nil {
		return ErrInvalidInvitation
	}
	if inv.Status != model.InvitationPending || !inv.ExpiresAt.After(s.Now()) {
		return ErrInvalidInvitation
	}
	if inv.InvitedEmail != "" && !strings.EqualFold(inv.InvitedEmail, email) {
		return ErrEmailMismatch
	}

	userID, err := s.Users.CreateUser(ctx, model.User{
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
	if err := s.Roles.GrantRole(ctx, model.RoleAssignment{
		UserID:        userID,
		CondominiumID: inv.CondominiumID,
		Role:          inv.InvitedRole,
	}); err != nil {
		return err
	}
	return s.Invitations.MarkInvitationAccepted(ctx, inv.ID)
}
