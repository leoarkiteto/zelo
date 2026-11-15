package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/repositories"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// SupplierService implements the supplier use cases (FR-5, FR-6).
type SupplierService struct {
	Suppliers SupplierRepo
	Audit     AuditRecorder
	Now       func() time.Time
}

func recordAudit(ctx context.Context, a AuditRecorder, actor string, ev model.AuditEventType, details map[string]any) {
	if a == nil {
		return
	}
	uid := actor
	// The write is already committed: a failed audit row is logged, not
	// surfaced as a failed request.
	if err := a.RecordEvent(ctx, &uid, ev, details); err != nil {
		slog.Error("invoices: record audit event", "event", string(ev), "error", err)
	}
}

// prepare normalizes and validates a supplier. The country defaults to PT.
func prepareSupplier(condoID string, s domain.Supplier) (domain.Supplier, error) {
	s.CondominiumID = condoID
	s.Country = strings.ToUpper(strings.TrimSpace(s.Country))
	if s.Country == "" {
		s.Country = domain.CountryPT
	}
	s.Name = strings.TrimSpace(s.Name)
	s.NIF = domain.NormalizeNIF(s.Country, s.NIF)
	if errs := domain.ValidateSupplier(s); len(errs) > 0 {
		return s, errs
	}
	return s, nil
}

func mapSupplierErr(err error) error {
	if errors.Is(err, repositories.ErrDuplicateNIF) {
		return domain.FieldErrors{"nif": KeyDuplicateNIF}
	}
	return err
}

// Create validates and stores a supplier of the condominium.
func (s *SupplierService) Create(ctx context.Context, actorID, condoID string, in domain.Supplier) (domain.Supplier, error) {
	sup, err := prepareSupplier(condoID, in)
	if err != nil {
		return domain.Supplier{}, err
	}
	out, err := s.Suppliers.CreateSupplier(ctx, sup, actorID)
	if err != nil {
		return domain.Supplier{}, mapSupplierErr(err)
	}
	recordAudit(ctx, s.Audit, actorID, model.AuditSupplierCreated,
		map[string]any{"condominium_id": condoID, "supplier_id": out.ID})
	return out, nil
}

// Update validates and replaces the editable fields of a supplier of the
// condominium. ErrNotFound when it is not in the condominium.
func (s *SupplierService) Update(ctx context.Context, actorID, condoID, id string, in domain.Supplier) error {
	if _, err := s.Suppliers.GetSupplier(ctx, condoID, id); err != nil {
		return err
	}
	in.ID = id
	sup, err := prepareSupplier(condoID, in)
	if err != nil {
		return err
	}
	if err := s.Suppliers.UpdateSupplier(ctx, sup, actorID); err != nil {
		return mapSupplierErr(err)
	}
	recordAudit(ctx, s.Audit, actorID, model.AuditSupplierUpdated,
		map[string]any{"condominium_id": condoID, "supplier_id": id})
	return nil
}

// Get returns a supplier of the condominium, or ErrNotFound.
func (s *SupplierService) Get(ctx context.Context, condoID, id string) (domain.Supplier, error) {
	return s.Suppliers.GetSupplier(ctx, condoID, id)
}

// Search lists suppliers of the condominium whose name or NIF contains q. An
// empty q lists all. limit <= 0 means the repository default.
func (s *SupplierService) Search(ctx context.Context, condoID, q string, limit int) ([]domain.Supplier, error) {
	return s.Suppliers.SearchSuppliers(ctx, condoID, q, limit)
}
