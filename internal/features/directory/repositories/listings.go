package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/leoarkiteto/zelo/internal/features/directory/domain"
	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
)

// ListingStore persists service provider listings.
type ListingStore struct {
	db *sql.DB
}

// NewListingStore creates a ListingStore.
func NewListingStore(db *sql.DB) *ListingStore { return &ListingStore{db: db} }

// CreateListing inserts a listing.
func (s *ListingStore) CreateListing(
	ctx context.Context,
	l domain.ServiceProviderListing,
) (string, error) {
	row := s.db.QueryRowContext(
		ctx,
		`
		INSERT INTO service_provider_listings
			(condominium_id, category_id, name, phone, phone_digits, notes,
			 recommended_by_unit_id, recommended_by_unit_code, submitted_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`,
		l.CondominiumID,
		l.CategoryID,
		l.Name,
		l.Phone,
		l.PhoneDigits,
		sharedstore.NullString(l.Notes),
		sharedstore.NullString(l.RecommendedByUnitID),
		l.RecommendedByUnitCode,
		l.SubmittedBy,
	)
	var id string
	if err := row.Scan(&id, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return "", fmt.Errorf("create listing: %w", err)
	}
	return id, nil
}

// UpdateListing replaces a listing's editable fields.
func (s *ListingStore) UpdateListing(ctx context.Context, l domain.ServiceProviderListing) error {
	res, err := s.db.ExecContext(
		ctx,
		`
		UPDATE service_provider_listings
		SET category_id = $1, name = $2, phone = $3, phone_digits = $4, notes = $5,
			updated_at = now()
		WHERE id = $6 AND condominium_id = $7`,
		l.CategoryID,
		l.Name,
		l.Phone,
		l.PhoneDigits,
		sharedstore.NullString(l.Notes),
		l.ID,
		l.CondominiumID,
	)
	if err != nil {
		return fmt.Errorf("update listing: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sharedstore.ErrNotFound
	}
	return nil
}

// DeleteListing removes a listing.
func (s *ListingStore) DeleteListing(ctx context.Context, id, condominiumID string) error {
	res, err := s.db.ExecContext(
		ctx,
		`DELETE FROM service_provider_listings WHERE id = $1 AND condominium_id = $2`,
		id,
		condominiumID,
	)
	if err != nil {
		return fmt.Errorf("delete listing: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sharedstore.ErrNotFound
	}
	return nil
}

// GetListingByID returns a listing with its category name.
func (s *ListingStore) GetListingByID(
	ctx context.Context,
	id string,
) (domain.ServiceProviderListing, error) {
	var l domain.ServiceProviderListing
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
		return domain.ServiceProviderListing{}, sharedstore.ErrNotFound
	}
	if err != nil {
		return domain.ServiceProviderListing{}, fmt.Errorf("get listing: %w", err)
	}
	l.Notes = notes.String
	l.RecommendedByUnitID = unitID.String
	return l, nil
}

// ListListings returns listings for a condominium, optionally filtered by an
// escaped keyword query across name/category/notes and by category id.
func (s *ListingStore) ListListings(
	ctx context.Context,
	condominiumID, query, categoryID string,
) ([]domain.ServiceProviderListing, error) {
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
		where = append(
			where,
			fmt.Sprintf(
				"(l.name ILIKE $%d ESCAPE '\\' OR c.name ILIKE $%d ESCAPE '\\' OR COALESCE(l.notes, '') ILIKE $%d ESCAPE '\\')",
				next,
				next,
				next,
			),
		)
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
	var out []domain.ServiceProviderListing
	for rows.Next() {
		var l domain.ServiceProviderListing
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
func (s *ListingStore) FindDuplicatePhone(
	ctx context.Context,
	condominiumID, phoneDigits, excludeID string,
) (bool, error) {
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
