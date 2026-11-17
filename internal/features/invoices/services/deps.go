// Package services implements the invoices use cases: suppliers, invoice
// drafts, registration, edits, payments, annulment and attachments. Services
// are plain structs; the narrow interfaces below are test seams that the
// concrete repository stores satisfy.
package services

import (
	"context"
	"errors"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/repositories"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/pagination"
)

// SupplierRepo is the supplier persistence consumed by the services.
// *repositories.SupplierStore satisfies it.
type SupplierRepo interface {
	CreateSupplier(ctx context.Context, s domain.Supplier, actor string) (domain.Supplier, error)
	UpdateSupplier(ctx context.Context, s domain.Supplier, actor string) error
	GetSupplier(ctx context.Context, condoID, id string) (domain.Supplier, error)
	SearchSuppliers(ctx context.Context, condoID, q string, limit int) ([]domain.Supplier, error)
	SearchSuppliersPage(ctx context.Context, condoID, q string, requested int) ([]domain.Supplier, pagination.Page, error)
}

// InvoiceRepo is the invoice persistence consumed by the services. Every write
// to an existing invoice runs its domain check under the invoice row lock.
// *repositories.InvoiceStore satisfies it.
type InvoiceRepo interface {
	SaveInvoice(ctx context.Context, inv domain.Invoice) (string, error)
	UpdateInvoice(ctx context.Context, inv domain.Invoice) error
	GetInvoice(ctx context.Context, condoID, id string) (domain.Invoice, error)
	List(ctx context.Context, condoID string, f domain.ListFilter) (domain.ListResult, error)
	FindByIdempotencyKey(ctx context.Context, condoID, key string) (string, error)
	FindDuplicate(ctx context.Context, condoID, supplierID string, docType domain.DocumentType, number, excludeID string) (string, error)
	AddPayment(ctx context.Context, condoID, invoiceID string, p domain.Payment) (domain.Payment, error)
	Annul(ctx context.Context, condoID, invoiceID, reason, actor string) error
	SoftDeleteDraft(ctx context.Context, condoID, invoiceID, actor string) error
	AddAttachments(ctx context.Context, condoID, invoiceID string, atts []domain.Attachment) ([]domain.Attachment, error)
	RemoveAttachment(ctx context.Context, condoID, invoiceID, attID string) (domain.Attachment, error)
	GetAttachment(ctx context.Context, condoID, invoiceID, attID string) (domain.Attachment, error)
}

// AuditRecorder records security-relevant events. *store.AuditStore satisfies it.
type AuditRecorder interface {
	RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
}

// Compile-time checks that the concrete stores satisfy the seams.
var (
	_ SupplierRepo = (*repositories.SupplierStore)(nil)
	_ InvoiceRepo  = (*repositories.InvoiceStore)(nil)
)

// i18n keys of the field errors produced by the services themselves. The
// catalog must provide KeyDuplicateNIF; KeyUnknownSupplier already exists.
const (
	// KeyUnknownSupplier: the supplier id does not exist in the condominium.
	KeyUnknownSupplier = "suppliers.error.not_found"
	// KeyDuplicateNIF: the NIF is already used by another supplier of the condominium.
	KeyDuplicateNIF = "suppliers.error.duplicate_nif"
)

// Service errors surfaced to handlers. Domain rule errors (ErrOverpayment,
// ErrAnnulled, ...) and domain.FieldErrors are returned unchanged.
var (
	// ErrNotFound: the record does not exist in the condominium (maps to 404).
	ErrNotFound = repositories.ErrNotFound
	// ErrDuplicateInvoice matches *DuplicateInvoiceError with errors.Is.
	ErrDuplicateInvoice = repositories.ErrDuplicateInvoice
	// ErrNotDraft: the operation only applies to a draft.
	ErrNotDraft = repositories.ErrNotDraft
	// ErrInvalidTransition: the invoice changed state concurrently (for
	// example a draft was registered by another request); maps to 409.
	ErrInvalidTransition = repositories.ErrInvalidTransition
	// ErrInvalidFile: an uploaded file is not a PDF, JPEG or PNG, or is too large.
	ErrInvalidFile = errors.New("invalid attachment file")
	// ErrNoFiles: an attachment upload carried no file.
	ErrNoFiles = errors.New("no attachment files")
)

// DuplicateInvoiceError reports that (condominium, supplier, document type,
// number) already exists (FR-23). ExistingID lets the handler link to it.
type DuplicateInvoiceError struct{ ExistingID string }

func (e *DuplicateInvoiceError) Error() string { return ErrDuplicateInvoice.Error() }

// Is makes errors.Is(err, ErrDuplicateInvoice) true.
func (e *DuplicateInvoiceError) Is(target error) bool { return target == ErrDuplicateInvoice }

// SaveResult is the outcome of creating an invoice. Existing is true when the
// idempotency key had already created the invoice (FR-24): the handler should
// redirect to ID instead of reporting a new creation.
type SaveResult struct {
	ID       string
	Existing bool
}
