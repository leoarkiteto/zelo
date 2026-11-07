// Package handlers contains directory HTTP handlers and route registration.
package handlers

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/directory/domain"
	"github.com/leoarkiteto/zelo/internal/features/directory/repositories"
	"github.com/leoarkiteto/zelo/internal/features/directory/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
)

type ListingService interface {
	CreateListing(
		ctx context.Context,
		residentID, condominiumID string,
		in services.ListingInput) (domain.ServiceProviderListing, bool, error)

	SearchListings(
		ctx context.Context,
		condominiumID, query, categoryID string,
	) ([]domain.ServiceProviderListing, error)

	EditListing(
		ctx context.Context,
		actorID, listingID, condominiumID string,
		in services.ListingInput,
	) error

	DeleteListing(
		ctx context.Context,
		actorID, listingID, condominiumID string,
	) error
}

type CategoryService interface {
	CreateCategory(
		ctx context.Context,
		actorID, condominiumID, name string,
	) (string, error)

	RenameCategory(
		ctx context.Context,
		actorID, categoryID, condominiumID, name string,
	) error

	DeactivateCategory(
		ctx context.Context,
		actorID, categoryID, condominiumID string,
	) error
}

// Deps are the collaborators used by directory handlers.
type Deps struct {
	Roles               middleware.RoleChecker
	Audit               middleware.AuditRecorder
	Listings            *repositories.ListingStore
	Categories          *repositories.CategoryStore
	DirectoryListings   ListingService
	DirectoryCategories CategoryService
}

// Handler bundles dependencies for directory handler methods.
type Handler struct {
	deps Deps
}
