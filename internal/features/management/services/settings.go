package services

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/percent"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

var (
	// ErrInvalidCurrency is returned when the requested currency is not supported.
	ErrInvalidCurrency = errors.New("invalid currency")
	// ErrInvalidFee is returned for a multa that is not a 0..100 % value with at most two decimals.
	ErrInvalidFee = errors.New("invalid late fee")
	// ErrInvalidInterest is returned for a juros value that is not 0..100 % with at most two decimals.
	ErrInvalidInterest = errors.New("invalid late interest")
	// ErrInvalidGrace is returned for grace days outside 0..60.
	ErrInvalidGrace = errors.New("invalid grace days")
	// ErrFeeAboveBRLCap is returned when a BRL condominium sets a multa above 2 %.
	ErrFeeAboveBRLCap = errors.New("late fee above BRL cap")
)

// MaxGraceDays is the largest accepted grace period.
const MaxGraceDays = 60

// CondominiumSettingsStore reads and updates condominium settings.
// *store.CondominiumStore satisfies it.
type CondominiumSettingsStore interface {
	GetCurrency(ctx context.Context, condoID string) (money.Currency, error)
	GetLateFeeSettings(ctx context.Context, condoID string) (model.LateFeeSettings, error)
	SetLateFeeSettings(ctx context.Context, condoID string, v model.LateFeeSettings, maxBRLFeeBP int) error
	SetCurrencyClampingLateFee(ctx context.Context, condoID string, c money.Currency, maxBRLFeeBP int) error
}

// SettingsService implements the condominium settings use cases (FR-2, FR-3, FR-32).
type SettingsService struct {
	Condominiums CondominiumSettingsStore
	Audit        AuditRecorder
}

// ChangeCurrency validates and stores the condominium currency in a single
// store call that also clamps a stored multa above the BRL cap, then records a
// condominium_currency_changed audit event (and late_fee_settings_changed when
// the multa was clamped). Existing amounts are not converted. Selecting the
// current currency is a no-op and records no event.
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
	oldFee, err := s.Condominiums.GetLateFeeSettings(ctx, condoID)
	if err != nil {
		return err
	}
	if err := s.Condominiums.SetCurrencyClampingLateFee(ctx, condoID, c, model.MaxBRLLateFeeBP); err != nil {
		return err
	}
	// The write is committed: failed audit rows are logged, not surfaced.
	uid := actorID
	s.record(ctx, &uid, model.AuditCondominiumCurrencyChanged, map[string]any{
		"condominium_id": condoID,
		"old_currency":   string(old),
		"new_currency":   string(c),
	})
	if c == money.BRL && oldFee.FeeBP > model.MaxBRLLateFeeBP {
		newFee := oldFee
		newFee.FeeBP = model.MaxBRLLateFeeBP
		s.record(ctx, &uid, model.AuditLateFeeSettingsChanged, lateFeeDetails(condoID, oldFee, newFee, "currency_change_clamp"))
	}
	return nil
}

// LateFeeInput is the raw form text of the late-fee settings.
type LateFeeInput struct {
	Fee, Interest, GraceDays string
}

// SaveLateFee validates and stores the multa, juros and grace days, and
// records a late_fee_settings_changed audit event when something changed.
func (s *SettingsService) SaveLateFee(ctx context.Context, actorID, condoID string, in LateFeeInput) error {
	feeBP, err := percent.ParseBP(in.Fee)
	if err != nil {
		return ErrInvalidFee
	}
	interestBP, err := percent.ParseBP(in.Interest)
	if err != nil {
		return ErrInvalidInterest
	}
	grace, err := strconv.Atoi(strings.TrimSpace(in.GraceDays))
	if err != nil || grace < 0 || grace > MaxGraceDays {
		return ErrInvalidGrace
	}
	old, err := s.Condominiums.GetLateFeeSettings(ctx, condoID)
	if err != nil {
		return err
	}
	next := model.LateFeeSettings{FeeBP: feeBP, InterestBP: interestBP, GraceDays: grace}
	if err := s.Condominiums.SetLateFeeSettings(ctx, condoID, next, model.MaxBRLLateFeeBP); err != nil {
		if errors.Is(err, store.ErrLateFeeAboveBRLCap) {
			return ErrFeeAboveBRLCap
		}
		return err
	}
	if old != next {
		uid := actorID
		s.record(ctx, &uid, model.AuditLateFeeSettingsChanged, lateFeeDetails(condoID, old, next, "settings"))
	}
	return nil
}

func (s *SettingsService) record(ctx context.Context, uid *string, t model.AuditEventType, details map[string]any) {
	if s.Audit == nil {
		return
	}
	if err := s.Audit.RecordEvent(ctx, uid, t, details); err != nil {
		slog.Error("management: record settings audit", "event", string(t), "error", err)
	}
}

func lateFeeDetails(condoID string, old, next model.LateFeeSettings, source string) map[string]any {
	return map[string]any{
		"condominium_id":  condoID,
		"source":          source,
		"old_fee_bp":      old.FeeBP,
		"new_fee_bp":      next.FeeBP,
		"old_interest_bp": old.InterestBP,
		"new_interest_bp": next.InterestBP,
		"old_grace_days":  old.GraceDays,
		"new_grace_days":  next.GraceDays,
	}
}
