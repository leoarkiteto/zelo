package ports

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// RoleStore is the persistence port for role assignments.
type RoleStore interface {
	GrantRole(ctx context.Context, a model.RoleAssignment) error
	RevokeRole(ctx context.Context, userID, condominiumID string, role model.Role) error
	ActiveRolesForUser(ctx context.Context, userID, condominiumID string) ([]model.Role, error)
	ActiveSyndicForCondominium(ctx context.Context, condominiumID string) (model.RoleAssignment, error)
	FirstActiveRoleForUser(ctx context.Context, userID string) (model.RoleAssignment, error)
	HasActiveRole(ctx context.Context, userID, condominiumID string, role model.Role) (bool, error)
}

// InvitationStore is the persistence port for invitations.
type InvitationStore interface {
	CreateInvitation(ctx context.Context, inv model.Invitation) (string, error)
	GetInvitationByTokenHash(ctx context.Context, tokenHash string) (model.Invitation, error)
	MarkInvitationAccepted(ctx context.Context, id string) error
	RevokeInvitation(ctx context.Context, id string) error
	ListInvitationsForCondominium(ctx context.Context, condominiumID string) ([]model.Invitation, error)
}

// UnitReader lists a condominium's units.
type UnitReader interface {
	ListUnitsForCondominium(ctx context.Context, condominiumID string) ([]model.Unit, error)
}

// AuditRecorder records security-relevant events.
type AuditRecorder interface {
	RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
}
