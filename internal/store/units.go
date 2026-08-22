package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/leoarkiteto/zelo/internal/model"
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
