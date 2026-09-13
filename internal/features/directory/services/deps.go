package services

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/directory/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// ListingStore is the persistence port for directory listings.
type ListingStore interface {
	CreateListing(ctx context.Context, l domain.ServiceProviderListing) (string, error)
	UpdateListing(ctx context.Context, l domain.ServiceProviderListing) error
	DeleteListing(ctx context.Context, id, condominiumID string) error
	GetListingByID(ctx context.Context, id string) (domain.ServiceProviderListing, error)
	ListListings(
		ctx context.Context,
		condominiumID, query, categoryID string,
	) ([]domain.ServiceProviderListing, error)
	FindDuplicatePhone(
		ctx context.Context,
		condominiumID, phoneDigits, excludeID string,
	) (bool, error)
}

// CategoryStore is the persistence port for directory categories.
type CategoryStore interface {
	CreateCategory(ctx context.Context, c domain.ServiceCategory) (string, error)
	RenameCategory(ctx context.Context, id, name string) error
	DeactivateCategory(ctx context.Context, id string) error
	GetCategoryByID(ctx context.Context, id string) (domain.ServiceCategory, error)
	ListActiveCategories(
		ctx context.Context,
		condominiumID string,
	) ([]domain.ServiceCategory, error)
	ListCategories(ctx context.Context, condominiumID string) ([]domain.ServiceCategory, error)
}

// UnitResolver finds a user's active unit in a condominium.
type UnitResolver interface {
	GetActiveUnitForUser(ctx context.Context, userID, condominiumID string) (model.Unit, error)
}

// AuditRecorder records security-relevant events.
type AuditRecorder interface {
	RecordEvent(
		ctx context.Context,
		userID *string,
		eventType model.AuditEventType,
		details map[string]any,
	) error
}
