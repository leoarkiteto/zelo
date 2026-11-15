package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// ErrInvalidCurrency is returned when the requested currency is not supported.
var ErrInvalidCurrency = errors.New("invalid currency")

// CondominiumSettingsStore reads and updates condominium settings.
// *store.CondominiumStore satisfies it.
type CondominiumSettingsStore interface {
	GetCurrency(ctx context.Context, condoID string) (money.Currency, error)
	SetCurrency(ctx context.Context, condoID string, c money.Currency) error
}

// SettingsService implements the condominium settings use cases (FR-2, FR-32).
type SettingsService struct {
	Condominiums CondominiumSettingsStore
	Audit        AuditRecorder
}

// ChangeCurrency validates and stores the condominium currency, then records a
// condominium_currency_changed audit event with the old and new currency.
// Existing amounts are not converted. Selecting the current currency is a
// no-op and records no event.
func (s *SettingsService) ChangeCurrency(ctx context.Context, actorID, condoID string, c money.Currency) error {
	if !c.Valid() {
		return ErrInvalidCurrency
	}
	old, err := s.Condominiums.GetCurrency(ctx, condoID)
	if err != nil {
		return err
	}
	if old == c {
		return nil
	}
	if err := s.Condominiums.SetCurrency(ctx, condoID, c); err != nil {
		return err
	}
	if s.Audit != nil {
		uid := actorID
		// The write is committed: a failed audit row is logged, not surfaced.
		if err := s.Audit.RecordEvent(ctx, &uid, model.AuditCondominiumCurrencyChanged, map[string]any{
			"condominium_id": condoID,
			"old_currency":   string(old),
			"new_currency":   string(c),
		}); err != nil {
			slog.Error("management: record currency change", "error", err)
		}
	}
	return nil
}
