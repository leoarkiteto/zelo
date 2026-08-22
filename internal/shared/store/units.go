package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// UnitStore persists units.
type UnitStore struct {
	db *sql.DB
}

// NewUnitStore creates a UnitStore.
func NewUnitStore(db *sql.DB) *UnitStore { return &UnitStore{db: db} }

// ListUnitsForCondominium returns the units of a condominium.
func (s *UnitStore) ListUnitsForCondominium(ctx context.Context, condominiumID string) ([]model.Unit, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, condominium_id, code FROM units WHERE condominium_id = $1 ORDER BY code`, condominiumID)
	if err != nil {
		return nil, fmt.Errorf("list units: %w", err)
	}
	defer rows.Close()
	var units []model.Unit
	for rows.Next() {
		var u model.Unit
		if err := rows.Scan(&u.ID, &u.CondominiumID, &u.Code); err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	return units, rows.Err()
}

// GetActiveUnitForUser returns the user's active unit in a condominium. A user
// with no active occupancy, or with more than one, yields ErrNotFound.
func (s *UnitStore) GetActiveUnitForUser(ctx context.Context, userID, condominiumID string) (model.Unit, error) {
	var u model.Unit
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.condominium_id, u.code
		FROM unit_occupancies occ
		JOIN units u ON u.id = occ.unit_id
		WHERE occ.user_id = $1 AND u.condominium_id = $2 AND occ.ended_at IS NULL
		LIMIT 1`, userID, condominiumID).Scan(&u.ID, &u.CondominiumID, &u.Code)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Unit{}, ErrNotFound
	}
	if err != nil {
		return model.Unit{}, fmt.Errorf("get active unit for user: %w", err)
	}
	return u, nil
}
