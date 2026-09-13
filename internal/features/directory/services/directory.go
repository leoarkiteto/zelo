package services

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/leoarkiteto/zelo/internal/features/directory/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// Directory service errors.
var (
	// ErrInvalidListing is returned when listing fields fail validation.
	ErrInvalidListing = errors.New("invalid listing")
	// ErrInvalidPhone is returned when the phone number is malformed.
	ErrInvalidPhone = errors.New("invalid phone number")
	// ErrInvalidCategory is returned when the category is unknown, inactive,
	// or belongs to another condominium.
	ErrInvalidCategory = errors.New("invalid category")
	// ErrNoUnit is returned when the resident has no active unit.
	ErrNoUnit = errors.New("resident has no active unit")
	// ErrNotFound aliases the domain-level not-found sentinel.
	ErrNotFound = model.ErrNotFound
	// ErrDuplicate aliases the domain-level duplicate sentinel.
	ErrDuplicate = model.ErrDuplicate
)

const (
	maxListingNameLen  = 120
	maxListingNotesLen = 500
	maxCategoryNameLen = 80
	minPhoneDigits     = 7
	maxPhoneDigits     = 15
)

// ListingInput is a validated listing submission or edit payload.
type ListingInput struct {
	Name             string
	CategoryID       string
	Phone            string
	Notes            string
	ConfirmDuplicate bool
}

// DirectoryService implements directory use cases (US1-US3).
type DirectoryService struct {
	Listings   ListingStore
	Categories CategoryStore
	Units      UnitResolver
	Audit      AuditRecorder
	Now        func() time.Time
}

// CreateListing validates and stores a resident's recommended professional.
// It returns duplicate=true when a listing with the same phone number already
// exists and the submitter has not confirmed saving anyway (spec FR-021).
func (s *DirectoryService) CreateListing(
	ctx context.Context,
	residentID, condominiumID string,
	in ListingInput,
) (domain.ServiceProviderListing, bool, error) {
	if err := s.validateListingInput(in); err != nil {
		return domain.ServiceProviderListing{}, false, err
	}
	category, err := s.requireActiveCategory(ctx, condominiumID, in.CategoryID)
	if err != nil {
		return domain.ServiceProviderListing{}, false, err
	}
	unit, err := s.Units.GetActiveUnitForUser(ctx, residentID, condominiumID)
	if err != nil {
		return domain.ServiceProviderListing{}, false, ErrNoUnit
	}
	digits := phoneDigits(in.Phone)
	duplicate, err := s.Listings.FindDuplicatePhone(ctx, condominiumID, digits, "")
	if err != nil {
		return domain.ServiceProviderListing{}, false, err
	}
	if duplicate && !in.ConfirmDuplicate {
		return domain.ServiceProviderListing{}, true, nil
	}

	listing := domain.ServiceProviderListing{
		CondominiumID:         condominiumID,
		CategoryID:            category.ID,
		CategoryName:          category.Name,
		Name:                  strings.TrimSpace(in.Name),
		Phone:                 strings.TrimSpace(in.Phone),
		PhoneDigits:           digits,
		Notes:                 strings.TrimSpace(in.Notes),
		RecommendedByUnitID:   unit.ID,
		RecommendedByUnitCode: unit.Code,
		SubmittedBy:           residentID,
	}
	id, err := s.Listings.CreateListing(ctx, listing)
	if err != nil {
		return domain.ServiceProviderListing{}, false, err
	}
	listing.ID = id
	uid := residentID
	if err := s.Audit.RecordEvent(ctx, &uid, model.AuditListingCreated,
		map[string]any{"listing_id": id, "condominium_id": condominiumID}); err != nil {
		return domain.ServiceProviderListing{}, false, err
	}
	return listing, false, nil
}

// SearchListings returns listings matching an optional keyword and category.
func (s *DirectoryService) SearchListings(
	ctx context.Context,
	condominiumID, query, categoryID string,
) ([]domain.ServiceProviderListing, error) {
	if categoryID != "" {
		if _, err := s.requireActiveCategory(ctx, condominiumID, categoryID); err != nil {
			return nil, err
		}
	}
	return s.Listings.ListListings(ctx, condominiumID, strings.TrimSpace(query), categoryID)
}

// EditListing updates a listing's editable fields (syndic moderation).
func (s *DirectoryService) EditListing(
	ctx context.Context,
	actorID, listingID, condominiumID string,
	in ListingInput,
) error {
	if err := s.validateListingInput(in); err != nil {
		return err
	}
	category, err := s.requireActiveCategory(ctx, condominiumID, in.CategoryID)
	if err != nil {
		return err
	}
	existing, err := s.Listings.GetListingByID(ctx, listingID)
	if err != nil {
		return err
	}
	if existing.CondominiumID != condominiumID {
		return ErrNotFound
	}
	if err := s.Listings.UpdateListing(ctx, domain.ServiceProviderListing{
		ID:            listingID,
		CondominiumID: condominiumID,
		CategoryID:    category.ID,
		Name:          strings.TrimSpace(in.Name),
		Phone:         strings.TrimSpace(in.Phone),
		PhoneDigits:   phoneDigits(in.Phone),
		Notes:         strings.TrimSpace(in.Notes),
	}); err != nil {
		return err
	}
	uid := actorID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditListingEdited,
		map[string]any{"listing_id": listingID, "condominium_id": condominiumID})
}

// DeleteListing removes a listing (syndic moderation).
func (s *DirectoryService) DeleteListing(
	ctx context.Context,
	actorID, listingID, condominiumID string,
) error {
	existing, err := s.Listings.GetListingByID(ctx, listingID)
	if err != nil {
		return err
	}
	if existing.CondominiumID != condominiumID {
		return ErrNotFound
	}
	if err := s.Listings.DeleteListing(ctx, listingID, condominiumID); err != nil {
		return err
	}
	uid := actorID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditListingDeleted,
		map[string]any{"listing_id": listingID, "condominium_id": condominiumID})
}

// CreateCategory adds a category for a condominium.
func (s *DirectoryService) CreateCategory(
	ctx context.Context,
	actorID, condominiumID, name string,
) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > maxCategoryNameLen {
		return "", ErrInvalidListing
	}
	id, err := s.Categories.CreateCategory(
		ctx,
		domain.ServiceCategory{CondominiumID: condominiumID, Name: name},
	)
	if err != nil {
		return "", err
	}
	uid := actorID
	if err := s.Audit.RecordEvent(ctx, &uid, model.AuditCategoryCreated,
		map[string]any{"category_id": id, "condominium_id": condominiumID}); err != nil {
		return "", err
	}
	return id, nil
}

// RenameCategory renames a category in the condominium.
func (s *DirectoryService) RenameCategory(
	ctx context.Context,
	actorID, categoryID, condominiumID, name string,
) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > maxCategoryNameLen {
		return ErrInvalidListing
	}
	if err := s.requireCategoryInCondominium(ctx, categoryID, condominiumID); err != nil {
		return err
	}
	if err := s.Categories.RenameCategory(ctx, categoryID, name); err != nil {
		return err
	}
	uid := actorID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditCategoryRenamed,
		map[string]any{"category_id": categoryID, "condominium_id": condominiumID})
}

// DeactivateCategory deactivates a category in the condominium.
func (s *DirectoryService) DeactivateCategory(
	ctx context.Context,
	actorID, categoryID, condominiumID string,
) error {
	if err := s.requireCategoryInCondominium(ctx, categoryID, condominiumID); err != nil {
		return err
	}
	if err := s.Categories.DeactivateCategory(ctx, categoryID); err != nil {
		return err
	}
	uid := actorID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditCategoryDeactivated,
		map[string]any{"category_id": categoryID, "condominium_id": condominiumID})
}

func (s *DirectoryService) validateListingInput(in ListingInput) error {
	if strings.TrimSpace(in.Name) == "" || len([]rune(in.Name)) > maxListingNameLen {
		return ErrInvalidListing
	}
	if strings.TrimSpace(in.Notes) != "" && len([]rune(in.Notes)) > maxListingNotesLen {
		return ErrInvalidListing
	}
	if !validPhoneChars(in.Phone) {
		return ErrInvalidPhone
	}
	if digits := phoneDigits(in.Phone); len(digits) < minPhoneDigits ||
		len(digits) > maxPhoneDigits {
		return ErrInvalidPhone
	}
	return nil
}

func (s *DirectoryService) requireActiveCategory(
	ctx context.Context,
	condominiumID, categoryID string,
) (domain.ServiceCategory, error) {
	category, err := s.Categories.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return domain.ServiceCategory{}, ErrInvalidCategory
	}
	if category.CondominiumID != condominiumID || !category.Active {
		return domain.ServiceCategory{}, ErrInvalidCategory
	}
	return category, nil
}

func (s *DirectoryService) requireCategoryInCondominium(
	ctx context.Context,
	categoryID, condominiumID string,
) error {
	category, err := s.Categories.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return err
	}
	if category.CondominiumID != condominiumID {
		return ErrNotFound
	}
	return nil
}

// phoneDigits returns the digits-only normalization of a phone number.
func phoneDigits(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// validPhoneChars reports whether every rune is allowed in a phone number.
func validPhoneChars(phone string) bool {
	for _, r := range phone {
		if unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '+', ' ', '-', '(', ')', '.', '/':
			continue
		default:
			return false
		}
	}
	return true
}
