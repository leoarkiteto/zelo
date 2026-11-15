// Package repositories implements the invoices persistence with database/sql
// and plain SQL (constitution VI). Every query is scoped by condominium_id.
package repositories

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
)

// Typed repository errors, compared with errors.Is.
var (
	// ErrNotFound: no such record in the condominium (also a malformed id).
	ErrNotFound = sharedstore.ErrNotFound
	// ErrDuplicateInvoice: the (supplier, document type, number) key is taken
	// (FR-23). The concrete error is a *DuplicateInvoiceError.
	ErrDuplicateInvoice = errors.New("duplicate invoice")
	// ErrDuplicateNIF: the supplier NIF is taken in the condominium (FR-6).
	ErrDuplicateNIF = errors.New("duplicate supplier nif")
	// ErrDuplicateIdempotencyKey: the idempotency key was already used
	// (FR-24). The concrete error is a *DuplicateKeyError.
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
	// ErrInvalidReference: a foreign key was violated, e.g. a supplier of
	// another condominium.
	ErrInvalidReference = errors.New("invalid reference")
	// ErrNotDraft: the operation only applies to a draft.
	ErrNotDraft = errors.New("invoice is not a draft")
	// ErrInvalidTransition: the requested state change is not allowed
	// (registered back to draft, or anything to annulled through an edit).
	ErrInvalidTransition = errors.New("invalid state transition")
)

// DuplicateInvoiceError carries the id of the invoice that holds the key.
type DuplicateInvoiceError struct{ ExistingID string }

func (e *DuplicateInvoiceError) Error() string { return ErrDuplicateInvoice.Error() }

// Is makes errors.Is(err, ErrDuplicateInvoice) true.
func (e *DuplicateInvoiceError) Is(target error) bool { return target == ErrDuplicateInvoice }

// DuplicateKeyError carries the id of the invoice created with the same
// idempotency key.
type DuplicateKeyError struct{ ExistingID string }

func (e *DuplicateKeyError) Error() string { return ErrDuplicateIdempotencyKey.Error() }

// Is makes errors.Is(err, ErrDuplicateIdempotencyKey) true.
func (e *DuplicateKeyError) Is(target error) bool { return target == ErrDuplicateIdempotencyKey }

// Constraint names from migration 0015.
const (
	constraintSupplierNIF = "idx_suppliers_condo_nif"
	constraintDuplicate   = "idx_invoices_duplicate"
	constraintIdempotency = "invoices_idempotency_unique"
)

func pgError(err error) *pgconn.PgError {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe
	}
	return nil
}
