package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/leoarkiteto/zelo/internal/shared/model"
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

// GetLateFeeSettings returns the condominium's late-fee settings, or ErrNotFound.
func (s *CondominiumStore) GetLateFeeSettings(ctx context.Context, condoID string) (model.LateFeeSettings, error) {
	var out model.LateFeeSettings
	err := s.db.QueryRowContext(ctx,
		`SELECT late_fee_bp, late_interest_bp, late_grace_days FROM condominiums WHERE id = $1`, condoID).
		Scan(&out.FeeBP, &out.InterestBP, &out.GraceDays)
	if errors.Is(err, sql.ErrNoRows) {
		return model.LateFeeSettings{}, ErrNotFound
	}
	if err != nil {
		return model.LateFeeSettings{}, fmt.Errorf("get late fee settings: %w", err)
	}
	return out, nil
}

// SetLateFeeSettings stores the condominium's late-fee settings. Range checks
// are the database CHECKs. The BRL cap is enforced atomically in the UPDATE's
// WHERE clause against the row's current currency: a BRL condominium with a fee
// above maxBRLFeeBP is refused with ErrLateFeeAboveBRLCap and left unchanged.
// Returns ErrNotFound for an unknown condominium.
func (s *CondominiumStore) SetLateFeeSettings(ctx context.Context, condoID string, v model.LateFeeSettings, maxBRLFeeBP int) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE condominiums SET late_fee_bp = $2, late_interest_bp = $3, late_grace_days = $4
		 WHERE id = $1 AND (currency <> 'BRL' OR $2::int <= $5::int)`,
		condoID, v.FeeBP, v.InterestBP, v.GraceDays, maxBRLFeeBP)
	if err != nil {
		return fmt.Errorf("set late fee settings: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set late fee settings: %w", err)
	}
	if n > 0 {
		return nil
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM condominiums WHERE id = $1)`, condoID).Scan(&exists); err != nil {
		return fmt.Errorf("set late fee settings: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	return ErrLateFeeAboveBRLCap
}

// SetCurrencyClampingLateFee changes the currency and, in the same single
// UPDATE, lowers a stored late fee above maxBRLFeeBP when the new currency is
// BRL. Returns ErrNotFound for an unknown condominium.
func (s *CondominiumStore) SetCurrencyClampingLateFee(ctx context.Context, condoID string, c money.Currency, maxBRLFeeBP int) error {
	if !c.Valid() {
		return fmt.Errorf("invalid currency %q", c)
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE condominiums
		SET currency = $2,
		    late_fee_bp = CASE WHEN $2 = 'BRL' THEN LEAST(late_fee_bp, $3::int) ELSE late_fee_bp END
		WHERE id = $1`, condoID, string(c), maxBRLFeeBP)
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
