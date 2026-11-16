package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/permille"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

// MaxUnitCodeLen is the longest accepted unit code, in characters.
const MaxUnitCodeLen = 32

var (
	// ErrCodeRequired is returned for an empty unit code.
	ErrCodeRequired = errors.New("unit code required")
	// ErrCodeTooLong is returned for a unit code above MaxUnitCodeLen.
	ErrCodeTooLong = errors.New("unit code too long")
	// ErrDuplicateCode is returned when the code is already used.
	ErrDuplicateCode = errors.New("duplicate unit code")
	// ErrUnitNotFound is returned when the unit does not exist in the condominium.
	ErrUnitNotFound = errors.New("unit not found")
)

// UnitStore is the persistence used by UnitService. *store.UnitStore satisfies it.
type UnitStore interface {
	ListUnitsForCondominium(ctx context.Context, condominiumID string) ([]model.Unit, error)
	CreateUnit(ctx context.Context, condominiumID, code string, sharePPM int) (model.Unit, error)
	UpdateUnit(ctx context.Context, condominiumID, unitID, code string, sharePPM int) error
}

// UnitList is the units of a condominium with the sum of their shares.
type UnitList struct {
	Units []model.Unit
	// SumPPM is the sum of every share in ppm.
	SumPPM int
	// DiffPPM is 100% minus SumPPM: positive when shares are missing,
	// negative when they exceed 100%.
	DiffPPM int
}

// UnitService implements the Unidades use cases (FR-1, FR-2, FR-23).
type UnitService struct {
	Units UnitStore
	Audit AuditRecorder
}

// List returns the units with the sum of shares and the difference to 100%.
func (s *UnitService) List(ctx context.Context, condoID string) (UnitList, error) {
	units, err := s.Units.ListUnitsForCondominium(ctx, condoID)
	if err != nil {
		return UnitList{}, err
	}
	out := UnitList{Units: units}
	for _, u := range units {
		out.SumPPM += u.SharePPM
	}
	out.DiffPPM = permille.Total - out.SumPPM
	return out, nil
}

// Get returns one unit of the condominium, or ErrUnitNotFound.
func (s *UnitService) Get(ctx context.Context, condoID, unitID string) (model.Unit, error) {
	units, err := s.Units.ListUnitsForCondominium(ctx, condoID)
	if err != nil {
		return model.Unit{}, err
	}
	for _, u := range units {
		if u.ID == unitID {
			return u, nil
		}
	}
	return model.Unit{}, ErrUnitNotFound
}

// Create validates and stores a unit and records unit_created.
func (s *UnitService) Create(ctx context.Context, actorID, condoID, code, share string) (model.Unit, error) {
	code, ppm, err := validateUnit(code, share)
	if err != nil {
		return model.Unit{}, err
	}
	u, err := s.Units.CreateUnit(ctx, condoID, code, ppm)
	if err != nil {
		return model.Unit{}, mapStoreErr(err)
	}
	s.audit(ctx, actorID, model.AuditUnitCreated, map[string]any{
		"condominium_id": condoID, "unit_id": u.ID, "code": code, "share_ppm": ppm,
	})
	return u, nil
}

// Update validates and stores the new code and share and records unit_updated.
func (s *UnitService) Update(ctx context.Context, actorID, condoID, unitID, code, share string) error {
	code, ppm, err := validateUnit(code, share)
	if err != nil {
		return err
	}
	old, err := s.Get(ctx, condoID, unitID)
	if err != nil {
		return err
	}
	if err := s.Units.UpdateUnit(ctx, condoID, unitID, code, ppm); err != nil {
		return mapStoreErr(err)
	}
	s.audit(ctx, actorID, model.AuditUnitUpdated, map[string]any{
		"condominium_id": condoID, "unit_id": unitID,
		"old_code": old.Code, "new_code": code,
		"old_share_ppm": old.SharePPM, "new_share_ppm": ppm,
	})
	return nil
}

func (s *UnitService) audit(ctx context.Context, actorID string, t model.AuditEventType, details map[string]any) {
	if s.Audit == nil {
		return
	}
	uid := actorID
	// The write is committed: a failed audit row is logged, not surfaced.
	if err := s.Audit.RecordEvent(ctx, &uid, t, details); err != nil {
		slog.Error("management: record unit event", "event", string(t), "error", err)
	}
}

func mapStoreErr(err error) error {
	switch {
	case errors.Is(err, store.ErrConflict):
		return ErrDuplicateCode
	case errors.Is(err, store.ErrNotFound):
		return ErrUnitNotFound
	}
	return err
}

func validateUnit(code, share string) (string, int, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", 0, ErrCodeRequired
	}
	if utf8.RuneCountInString(code) > MaxUnitCodeLen {
		return "", 0, ErrCodeTooLong
	}
	ppm, err := permille.Parse(share)
	if err != nil {
		return "", 0, err
	}
	return code, ppm, nil
}
