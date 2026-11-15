package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// CondominiumStore reads and updates condominium settings.
type CondominiumStore struct {
	db *sql.DB
}

// NewCondominiumStore creates a CondominiumStore.
func NewCondominiumStore(db *sql.DB) *CondominiumStore { return &CondominiumStore{db: db} }

// GetCurrency returns the condominium's currency, or ErrNotFound.
func (s *CondominiumStore) GetCurrency(ctx context.Context, condoID string) (money.Currency, error) {
	var c string
	err := s.db.QueryRowContext(ctx, `SELECT currency FROM condominiums WHERE id = $1`, condoID).Scan(&c)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get condominium currency: %w", err)
	}
	return money.Currency(c), nil
}

// SetCurrency changes the condominium's currency, or returns ErrNotFound.
func (s *CondominiumStore) SetCurrency(ctx context.Context, condoID string, c money.Currency) error {
	if !c.Valid() {
		return fmt.Errorf("invalid currency %q", c)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE condominiums SET currency = $2 WHERE id = $1`, condoID, string(c))
	if err != nil {
		return fmt.Errorf("set condominium currency: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set condominium currency: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
