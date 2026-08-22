// Package repositories contains directory persistence adapters.
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
	"github.com/leoarkiteto/zelo/internal/shared/model"
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
		if sharedstore.IsUniqueViolation(err) {
			return "", sharedstore.ErrDuplicate
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
		if sharedstore.IsUniqueViolation(err) {
			return sharedstore.ErrDuplicate
		}
		return fmt.Errorf("rename category: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sharedstore.ErrNotFound
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
		return sharedstore.ErrNotFound
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
		return model.ServiceCategory{}, sharedstore.ErrNotFound
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
