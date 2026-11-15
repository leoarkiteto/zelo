// Package handlers contains the invoices HTTP handlers (invoices, suppliers
// and the HTMX fragments of the invoice form) and route registration.
package handlers

import (
	"context"
	"mime/multipart"
	"os"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
)

// Invoices is the invoice use-case surface the handlers depend on.
// *services.InvoiceService satisfies it.
type Invoices interface {
	SaveDraft(ctx context.Context, actorID, condoID string, inv domain.Invoice) (services.SaveResult, error)
	Register(ctx context.Context, actorID, condoID string, inv domain.Invoice) (services.SaveResult, error)
	Edit(ctx context.Context, actorID, condoID, id string, inv domain.Invoice, register bool) error
	DeleteDraft(ctx context.Context, actorID, condoID, id string) error
	Annul(ctx context.Context, actorID, condoID, id, reason string) error
	AddPayment(ctx context.Context, actorID, condoID, invoiceID string, p domain.Payment) (domain.Payment, error)
	AddAttachments(ctx context.Context, actorID, condoID, invoiceID string, files []*multipart.FileHeader) ([]domain.Attachment, error)
	RemoveAttachment(ctx context.Context, actorID, condoID, invoiceID, attID string) error
	OpenAttachment(ctx context.Context, condoID, invoiceID, attID string) (*os.File, domain.Attachment, error)
	Get(ctx context.Context, condoID, id string) (domain.Invoice, error)
	List(ctx context.Context, condoID string, f domain.ListFilter) (domain.ListResult, error)
	Preview(region domain.FiscalRegion, lines []domain.Line, withholdingCents int64) (domain.Totals, error)
}

// Suppliers is the supplier use-case surface the handlers depend on.
// *services.SupplierService satisfies it.
type Suppliers interface {
	Create(ctx context.Context, actorID, condoID string, in domain.Supplier) (domain.Supplier, error)
	Update(ctx context.Context, actorID, condoID, id string, in domain.Supplier) error
	Get(ctx context.Context, condoID, id string) (domain.Supplier, error)
	Search(ctx context.Context, condoID, q string, limit int) ([]domain.Supplier, error)
}

var (
	_ Invoices  = (*services.InvoiceService)(nil)
	_ Suppliers = (*services.SupplierService)(nil)
)

// Deps are the collaborators used by the invoices handlers.
type Deps struct {
	Roles     middleware.RoleChecker
	Audit     middleware.AuditRecorder
	Invoices  Invoices
	Suppliers Suppliers
}

// Handler bundles dependencies for the invoices handler methods.
type Handler struct {
	deps Deps
}
