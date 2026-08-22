package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/leoarkiteto/zelo/internal/model"
)

// CategoryStore persists service directory categories.
type CategoryStore struct {
	db *sql.DB
}

// NewCategoryStore creates a CategoryStore.
func NewCategoryStore(db *sql.DB) *CategoryStore { return &CategoryStore{db: db} }

// CreateCategory inserts a category.
func (s *CategoryStore) CreateCategory(ctx context.Context, c model.ServiceCategory) (string, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO service_categories (condominium_id, name)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`,
		c.CondominiumID, c.Name)
	var id string
	if err := row.Scan(&id, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return "", ErrDuplicate
		}
		return "", fmt.Errorf("create category: %w", err)
	}
	return id, nil
}

// RenameCategory renames a category.
func (s *CategoryStore) RenameCategory(ctx context.Context, id, name string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE service_categories SET name = $1, updated_at = now() WHERE id = $2`, name, id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("rename category: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeactivateCategory deactivates a category, keeping the row for history.
func (s *CategoryStore) DeactivateCategory(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE service_categories SET active = false, updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deactivate category: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetCategoryByID returns a category by id.
func (s *CategoryStore) GetCategoryByID(ctx context.Context, id string) (model.ServiceCategory, error) {
	var c model.ServiceCategory
	err := s.db.QueryRowContext(ctx, `
		SELECT id, condominium_id, name, active, created_at, updated_at
		FROM service_categories WHERE id = $1`, id).Scan(
		&c.ID, &c.CondominiumID, &c.Name, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ServiceCategory{}, ErrNotFound
	}
	if err != nil {
		return model.ServiceCategory{}, fmt.Errorf("get category: %w", err)
	}
	return c, nil
}

// ListActiveCategories returns active categories for a condominium.
func (s *CategoryStore) ListActiveCategories(ctx context.Context, condominiumID string) ([]model.ServiceCategory, error) {
	return s.listCategories(ctx, condominiumID, true)
}

// ListCategories returns all categories (active and deactivated) for a condominium.
func (s *CategoryStore) ListCategories(ctx context.Context, condominiumID string) ([]model.ServiceCategory, error) {
	return s.listCategories(ctx, condominiumID, false)
}

func (s *CategoryStore) listCategories(ctx context.Context, condominiumID string, activeOnly bool) ([]model.ServiceCategory, error) {
	query := `SELECT id, condominium_id, name, active, created_at, updated_at
		FROM service_categories WHERE condominium_id = $1`
	if activeOnly {
		query += ` AND active = TRUE`
	}
	query += ` ORDER BY name`
	rows, err := s.db.QueryContext(ctx, query, condominiumID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()
	var out []model.ServiceCategory
	for rows.Next() {
		var c model.ServiceCategory
		if err := rows.Scan(&c.ID, &c.CondominiumID, &c.Name, &c.Active, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListingStore persists service provider listings.
type ListingStore struct {
	db *sql.DB
}

// NewListingStore creates a ListingStore.
func NewListingStore(db *sql.DB) *ListingStore { return &ListingStore{db: db} }

// CreateListing inserts a listing.
func (s *ListingStore) CreateListing(ctx context.Context, l model.ServiceProviderListing) (string, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO service_provider_listings
			(condominium_id, category_id, name, phone, phone_digits, notes,
			 recommended_by_unit_id, recommended_by_unit_code, submitted_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`,
		l.CondominiumID, l.CategoryID, l.Name, l.Phone, l.PhoneDigits, nullString(l.Notes),
		nullString(l.RecommendedByUnitID), l.RecommendedByUnitCode, l.SubmittedBy)
	var id string
	if err := row.Scan(&id, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return "", fmt.Errorf("create listing: %w", err)
	}
	return id, nil
}

// UpdateListing replaces a listing's editable fields.
func (s *ListingStore) UpdateListing(ctx context.Context, l model.ServiceProviderListing) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE service_provider_listings
		SET category_id = $1, name = $2, phone = $3, phone_digits = $4, notes = $5,
			updated_at = now()
		WHERE id = $6 AND condominium_id = $7`,
		l.CategoryID, l.Name, l.Phone, l.PhoneDigits, nullString(l.Notes), l.ID, l.CondominiumID)
	if err != nil {
		return fmt.Errorf("update listing: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteListing removes a listing.
func (s *ListingStore) DeleteListing(ctx context.Context, id, condominiumID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM service_provider_listings WHERE id = $1 AND condominium_id = $2`, id, condominiumID)
	if err != nil {
		return fmt.Errorf("delete listing: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetListingByID returns a listing with its category name.
func (s *ListingStore) GetListingByID(ctx context.Context, id string) (model.ServiceProviderListing, error) {
	var l model.ServiceProviderListing
	var notes sql.NullString
	var unitID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT l.id, l.condominium_id, l.category_id, c.name, l.name, l.phone,
			l.phone_digits, l.notes, l.recommended_by_unit_id, l.recommended_by_unit_code,
			l.submitted_by, l.created_at, l.updated_at
		FROM service_provider_listings l
		JOIN service_categories c ON c.id = l.category_id
		WHERE l.id = $1`, id).Scan(
		&l.ID, &l.CondominiumID, &l.CategoryID, &l.CategoryName, &l.Name, &l.Phone,
		&l.PhoneDigits, &notes, &unitID, &l.RecommendedByUnitCode,
		&l.SubmittedBy, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ServiceProviderListing{}, ErrNotFound
	}
	if err != nil {
		return model.ServiceProviderListing{}, fmt.Errorf("get listing: %w", err)
	}
	l.Notes = notes.String
	l.RecommendedByUnitID = unitID.String
	return l, nil
}

// ListListings returns listings for a condominium, optionally filtered by an
// escaped keyword query across name/category/notes and by category id.
func (s *ListingStore) ListListings(ctx context.Context, condominiumID, query, categoryID string) ([]model.ServiceProviderListing, error) {
	params := []any{condominiumID}
	where := []string{"l.condominium_id = $1"}
	next := 2
	if categoryID != "" {
		where = append(where, fmt.Sprintf("l.category_id = $%d", next))
		params = append(params, categoryID)
		next++
	}
	if query != "" {
		pattern := "%" + escapeLike(query) + "%"
		where = append(where,
			fmt.Sprintf("(l.name ILIKE $%d ESCAPE '\\' OR c.name ILIKE $%d ESCAPE '\\' OR COALESCE(l.notes, '') ILIKE $%d ESCAPE '\\')", next, next, next))
		params = append(params, pattern)
		next++
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.id, l.condominium_id, l.category_id, c.name, l.name, l.phone,
			l.phone_digits, l.notes, l.recommended_by_unit_id, l.recommended_by_unit_code,
			l.submitted_by, l.created_at, l.updated_at
		FROM service_provider_listings l
		JOIN service_categories c ON c.id = l.category_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY l.name`, params...)
	if err != nil {
		return nil, fmt.Errorf("list listings: %w", err)
	}
	defer rows.Close()
	var out []model.ServiceProviderListing
	for rows.Next() {
		var l model.ServiceProviderListing
		var notes sql.NullString
		var unitID sql.NullString
		if err := rows.Scan(&l.ID, &l.CondominiumID, &l.CategoryID, &l.CategoryName, &l.Name,
			&l.Phone, &l.PhoneDigits, &notes, &unitID, &l.RecommendedByUnitCode,
			&l.SubmittedBy, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		l.Notes = notes.String
		l.RecommendedByUnitID = unitID.String
		out = append(out, l)
	}
	return out, rows.Err()
}

// FindDuplicatePhone reports whether another listing in the condominium has
// the same normalized phone digits. excludeID skips the listing being edited.
func (s *ListingStore) FindDuplicatePhone(ctx context.Context, condominiumID, phoneDigits, excludeID string) (bool, error) {
	query := `SELECT EXISTS(
		SELECT 1 FROM service_provider_listings
		WHERE condominium_id = $1 AND phone_digits = $2`
	params := []any{condominiumID, phoneDigits}
	if excludeID != "" {
		query += ` AND id <> $3`
		params = append(params, excludeID)
	}
	query += `)`
	var exists bool
	err := s.db.QueryRowContext(ctx, query, params...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("find duplicate phone: %w", err)
	}
	return exists, nil
}

// escapeLike escapes LIKE wildcards so user input is matched literally.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
