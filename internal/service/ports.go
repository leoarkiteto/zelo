// Package service contains application use cases (hexagonal core).
package service

import (
	"context"
	"time"

	"github.com/leoarkiteto/zelo/internal/model"
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

// InvitationStore is the persistence port for invitations.
type InvitationStore interface {
	CreateInvitation(ctx context.Context, inv model.Invitation) (string, error)
	GetInvitationByTokenHash(ctx context.Context, tokenHash string) (model.Invitation, error)
	MarkInvitationAccepted(ctx context.Context, id string) error
	RevokeInvitation(ctx context.Context, id string) error
}

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) (bool, error)
	ValidatePassword(password string) error
}

// TokenHasher hashes opaque tokens before storage.
type TokenHasher interface {
	HashToken(token string) string
}

// ErrDuplicate is the domain-level duplicate sentinel re-exported for
// registration use cases.
var ErrDuplicate = model.ErrDuplicate

// AuditRecorder records security-relevant events (FR-013).
type AuditRecorder interface {
	RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
}

// CategoryStore is the persistence port for directory categories.
type CategoryStore interface {
	CreateCategory(ctx context.Context, c model.ServiceCategory) (string, error)
	RenameCategory(ctx context.Context, id, name string) error
	DeactivateCategory(ctx context.Context, id string) error
	GetCategoryByID(ctx context.Context, id string) (model.ServiceCategory, error)
	ListActiveCategories(ctx context.Context, condominiumID string) ([]model.ServiceCategory, error)
	ListCategories(ctx context.Context, condominiumID string) ([]model.ServiceCategory, error)
}

// ListingStore is the persistence port for directory listings.
type ListingStore interface {
	CreateListing(ctx context.Context, l model.ServiceProviderListing) (string, error)
	UpdateListing(ctx context.Context, l model.ServiceProviderListing) error
	DeleteListing(ctx context.Context, id, condominiumID string) error
	GetListingByID(ctx context.Context, id string) (model.ServiceProviderListing, error)
	ListListings(ctx context.Context, condominiumID, query, categoryID string) ([]model.ServiceProviderListing, error)
	FindDuplicatePhone(ctx context.Context, condominiumID, phoneDigits, excludeID string) (bool, error)
}

// UnitResolver finds a user's active unit in a condominium.
type UnitResolver interface {
	GetActiveUnitForUser(ctx context.Context, userID, condominiumID string) (model.Unit, error)
}
