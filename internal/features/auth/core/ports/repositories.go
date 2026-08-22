package ports

import (
	"context"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// UserStore is the persistence port for users.
type UserStore interface {
	CreateUser(ctx context.Context, u model.User) (string, error)
	GetUserByEmail(ctx context.Context, email string) (model.User, error)
	GetUserByID(ctx context.Context, id string) (model.User, error)
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
	RecordFailedSignIn(ctx context.Context, userID string) error
	ApplyLock(ctx context.Context, userID string, until time.Time) error
	ClearLock(ctx context.Context, userID string) error
}

// RoleStore is the persistence port for role assignments.
type RoleStore interface {
	GrantRole(ctx context.Context, a model.RoleAssignment) error
	RevokeRole(ctx context.Context, userID, condominiumID string, role model.Role) error
	ActiveRolesForUser(ctx context.Context, userID, condominiumID string) ([]model.Role, error)
	ActiveSyndicForCondominium(ctx context.Context, condominiumID string) (model.RoleAssignment, error)
	FirstActiveRoleForUser(ctx context.Context, userID string) (model.RoleAssignment, error)
	HasActiveRole(ctx context.Context, userID, condominiumID string, role model.Role) (bool, error)
}

// InvitationStore is the persistence port for invitation acceptance.
type InvitationStore interface {
	GetInvitationByTokenHash(ctx context.Context, tokenHash string) (model.Invitation, error)
	MarkInvitationAccepted(ctx context.Context, id string) error
}

// AuditRecorder records security-relevant events.
type AuditRecorder interface {
	RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
}
