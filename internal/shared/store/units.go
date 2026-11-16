package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// ErrConflict indicates a uniqueness conflict (for units, a duplicate code in
// the condominium). It is the same sentinel as ErrDuplicate.
var ErrConflict = ErrDuplicate

// UnitStore persists units.
type UnitStore struct {
	db *sql.DB
}

// NewUnitStore creates a UnitStore.
func NewUnitStore(db *sql.DB) *UnitStore { return &UnitStore{db: db} }

// ListUnitsForCondominium returns the units of a condominium.
func (s *UnitStore) ListUnitsForCondominium(ctx context.Context, condominiumID string) ([]model.Unit, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, condominium_id, code, share_ppm FROM units WHERE condominium_id = $1 ORDER BY code`, condominiumID)
	if err != nil {
		return nil, fmt.Errorf("list units: %w", err)
	}
	defer rows.Close()
	var units []model.Unit
	for rows.Next() {
		var u model.Unit
		if err := rows.Scan(&u.ID, &u.CondominiumID, &u.Code, &u.SharePPM); err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	return units, rows.Err()
}

// GetActiveUnitForUser returns the user's active unit in a condominium. A user
// with no active occupancy yields ErrNotFound. With several active units the
// first by unit code is returned (deterministic).
func (s *UnitStore) GetActiveUnitForUser(ctx context.Context, userID, condominiumID string) (model.Unit, error) {
	var u model.Unit
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.condominium_id, u.code, u.share_ppm
		FROM unit_occupancies occ
		JOIN units u ON u.id = occ.unit_id
		WHERE occ.user_id = $1 AND u.condominium_id = $2 AND occ.ended_at IS NULL
		ORDER BY u.code, u.id
		LIMIT 1`, userID, condominiumID).Scan(&u.ID, &u.CondominiumID, &u.Code, &u.SharePPM)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Unit{}, ErrNotFound
	}
	if err != nil {
		return model.Unit{}, fmt.Errorf("get active unit for user: %w", err)
	}
	return u, nil
}

// ListActiveOccupanciesForUser returns the units the user currently occupies in
// a condominium, one row per unit (the earliest active occupancy), ordered by
// unit code.
func (s *UnitStore) ListActiveOccupanciesForUser(ctx context.Context, userID, condominiumID string) ([]model.ActiveUnit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, condominium_id, code, share_ppm, occupancy_type, started_at FROM (
			SELECT DISTINCT ON (u.id) u.id, u.condominium_id, u.code, u.share_ppm,
				occ.occupancy_type::text AS occupancy_type, occ.started_at
			FROM unit_occupancies occ
			JOIN units u ON u.id = occ.unit_id
			WHERE occ.user_id = $1 AND u.condominium_id = $2 AND occ.ended_at IS NULL
			ORDER BY u.id, occ.started_at, occ.occupancy_type::text
		) t
		ORDER BY code, id`, userID, condominiumID)
	if err != nil {
		return nil, fmt.Errorf("list active occupancies: %w", err)
	}
	defer rows.Close()
	var out []model.ActiveUnit
	for rows.Next() {
		var a model.ActiveUnit
		var typ string
		if err := rows.Scan(&a.Unit.ID, &a.Unit.CondominiumID, &a.Unit.Code, &a.Unit.SharePPM, &typ, &a.StartedAt); err != nil {
			return nil, err
		}
		a.OccupancyType = model.OccupancyType(typ)
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateUnit inserts a unit. A code already used in the condominium yields
// ErrConflict.
func (s *UnitStore) CreateUnit(ctx context.Context, condominiumID, code string, sharePPM int) (model.Unit, error) {
	u := model.Unit{CondominiumID: condominiumID, Code: code, SharePPM: sharePPM}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO units (condominium_id, code, share_ppm) VALUES ($1, $2, $3) RETURNING id`,
		condominiumID, code, sharePPM).Scan(&u.ID)
	if err != nil {
		return model.Unit{}, mapUnitErr("create unit", err)
	}
	return u, nil
}

// UpdateUnit changes a unit's code and share. A unit that does not exist in the
// condominium yields ErrNotFound; a duplicate code yields ErrConflict.
func (s *UnitStore) UpdateUnit(ctx context.Context, condominiumID, unitID, code string, sharePPM int) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE units SET code = $3, share_ppm = $4 WHERE id = $1 AND condominium_id = $2`,
		unitID, condominiumID, code, sharePPM)
	if err != nil {
		return mapUnitErr("update unit", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update unit: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// mapUnitErr maps PostgreSQL errors: unique violation to ErrConflict, a
// malformed UUID to ErrNotFound.
func mapUnitErr(op string, err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23505":
			return ErrConflict
		case "22P02":
			return ErrNotFound
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
