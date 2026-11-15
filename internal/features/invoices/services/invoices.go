package services

import (
	"context"
	"errors"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/repositories"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/uploads"
)

// attachmentsSubdir is the folder of UploadDir that holds invoice files, apart
// from the finance receipts.
const attachmentsSubdir = "invoices"

// InvoiceService implements the invoice use cases (FR-13 to FR-26). Every
// operation is scoped to the condominium id it receives; a record of another
// condominium is ErrNotFound.
type InvoiceService struct {
	Invoices  InvoiceRepo
	Suppliers SupplierRepo
	Audit     AuditRecorder
	// UploadDir is UPLOAD_DIR; files are stored under UploadDir/invoices.
	UploadDir string
	Now       func() time.Time
}

func (s *InvoiceService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *InvoiceService) attachmentsDir() string {
	return filepath.Join(s.UploadDir, attachmentsSubdir)
}

func regionOrDefault(r domain.FiscalRegion) domain.FiscalRegion {
	if r == "" {
		return domain.RegionContinental
	}
	return r
}

func addFieldErrors(dst domain.FieldErrors, err error) bool {
	var fe domain.FieldErrors
	if errors.As(err, &fe) {
		for k, v := range fe {
			dst.Add(k, v)
		}
		return true
	}
	return false
}

// Preview computes the totals of the form lines without persisting anything
// (FR-13). Invalid input returns domain.FieldErrors.
func (s *InvoiceService) Preview(region domain.FiscalRegion, lines []domain.Line, withholdingCents int64) (domain.Totals, error) {
	return domain.ComputeTotals(regionOrDefault(region), lines, withholdingCents)
}

// prepare normalizes inv, validates it as a draft or a registration, resolves
// the supplier within the condominium and sets the totals from the lines. The
// returned error is domain.FieldErrors for anything the user can fix.
func (s *InvoiceService) prepare(ctx context.Context, condoID string, inv domain.Invoice, register bool) (domain.Invoice, error) {
	inv.CondominiumID = condoID
	inv.Number = domain.NormalizeNumber(inv.Number)
	inv.FiscalRegion = regionOrDefault(inv.FiscalRegion)
	if register {
		inv.State = domain.StateRegistered
	} else {
		inv.State = domain.StateDraft
	}

	var errs domain.FieldErrors
	if register {
		errs = domain.ValidateRegistration(inv)
	} else {
		errs = domain.ValidateDraft(inv)
	}
	totals, terr := domain.ComputeTotals(inv.FiscalRegion, inv.Lines, inv.Totals.WithholdingCents)
	if terr != nil && !addFieldErrors(errs, terr) {
		return inv, terr
	}

	if strings.TrimSpace(inv.SupplierID) != "" {
		sup, err := s.Suppliers.GetSupplier(ctx, condoID, inv.SupplierID)
		switch {
		case errors.Is(err, repositories.ErrNotFound):
			errs.Add("supplier", KeyUnknownSupplier)
		case err != nil:
			return inv, err
		default:
			inv.Supplier = &sup
		}
	}
	if len(errs) > 0 {
		return inv, errs
	}
	inv.Totals = totals
	return inv, nil
}

// checkDuplicate returns a *DuplicateInvoiceError when another invoice of the
// condominium holds the (supplier, document type, number) key (FR-23).
func (s *InvoiceService) checkDuplicate(ctx context.Context, inv domain.Invoice) error {
	if inv.SupplierID == "" || inv.Number == "" {
		return nil
	}
	id, err := s.Invoices.FindDuplicate(ctx, inv.CondominiumID, inv.SupplierID, inv.DocumentType, inv.Number, inv.ID)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return &DuplicateInvoiceError{ExistingID: id}
}

// mapSaveErr turns repository errors of a save into service errors.
func mapSaveErr(err error) error {
	var dup *repositories.DuplicateInvoiceError
	if errors.As(err, &dup) {
		return &DuplicateInvoiceError{ExistingID: dup.ExistingID}
	}
	if errors.Is(err, repositories.ErrInvalidReference) {
		return domain.FieldErrors{"supplier": KeyUnknownSupplier}
	}
	return err
}

// SaveDraft creates a Rascunho (FR-20). It is idempotent by inv.IdempotencyKey.
func (s *InvoiceService) SaveDraft(ctx context.Context, actorID, condoID string, inv domain.Invoice) (SaveResult, error) {
	return s.create(ctx, actorID, condoID, inv, false)
}

// Register creates a registered invoice with the full validation (FR-21). It
// is idempotent by inv.IdempotencyKey (FR-24); a duplicate document key returns
// a *DuplicateInvoiceError (FR-23).
func (s *InvoiceService) Register(ctx context.Context, actorID, condoID string, inv domain.Invoice) (SaveResult, error) {
	return s.create(ctx, actorID, condoID, inv, true)
}

func (s *InvoiceService) create(ctx context.Context, actorID, condoID string, inv domain.Invoice, register bool) (SaveResult, error) {
	if inv.IdempotencyKey != "" {
		id, err := s.Invoices.FindByIdempotencyKey(ctx, condoID, inv.IdempotencyKey)
		if err == nil {
			return SaveResult{ID: id, Existing: true}, nil
		}
		if !errors.Is(err, repositories.ErrNotFound) {
			return SaveResult{}, err
		}
	}
	inv.ID = ""
	inv, err := s.prepare(ctx, condoID, inv, register)
	if err != nil {
		return SaveResult{}, err
	}
	if err := s.checkDuplicate(ctx, inv); err != nil {
		return SaveResult{}, err
	}
	inv.CreatedBy, inv.UpdatedBy = actorID, actorID

	id, err := s.Invoices.SaveInvoice(ctx, inv)
	if err != nil {
		var key *repositories.DuplicateKeyError
		if errors.As(err, &key) && key.ExistingID != "" {
			return SaveResult{ID: key.ExistingID, Existing: true}, nil
		}
		return SaveResult{}, mapSaveErr(err)
	}
	ev := model.AuditInvoiceCreated
	if register {
		ev = model.AuditInvoiceRegistered
	}
	recordAudit(ctx, s.Audit, actorID, ev, map[string]any{"condominium_id": condoID, "invoice_id": id})
	return SaveResult{ID: id}, nil
}

// Edit saves an edit of invoice id (FR-19, FR-20). An edit of a draft saves a
// draft unless register is true, which completes it into a registered invoice
// with the full validation. An edit of a registered invoice is always fully
// validated. An annulled invoice returns domain.ErrAnnulled, and a new valor a
// pagar below the amount already paid returns domain.ErrPayableBelowPaid.
func (s *InvoiceService) Edit(ctx context.Context, actorID, condoID, id string, inv domain.Invoice, register bool) error {
	existing, err := s.Invoices.GetInvoice(ctx, condoID, id)
	if err != nil {
		return err
	}
	if err := domain.CheckEditable(existing.State); err != nil {
		return err
	}
	wasDraft := existing.State == domain.StateDraft
	if !wasDraft {
		register = true
	}

	inv.ID = id
	inv, err = s.prepare(ctx, condoID, inv, register)
	if err != nil {
		return err
	}
	if err := domain.CheckEdit(inv.Totals.PayableCents, existing.PaidCents()); err != nil {
		return err
	}
	if err := s.checkDuplicate(ctx, inv); err != nil {
		return err
	}
	inv.IdempotencyKey = existing.IdempotencyKey
	inv.CreatedBy = existing.CreatedBy
	inv.UpdatedBy = actorID

	if err := s.Invoices.UpdateInvoice(ctx, inv); err != nil {
		return mapSaveErr(err)
	}
	ev := model.AuditInvoiceUpdated
	if wasDraft && register {
		ev = model.AuditInvoiceRegistered
	}
	recordAudit(ctx, s.Audit, actorID, ev, map[string]any{"condominium_id": condoID, "invoice_id": id})
	return nil
}

// DeleteDraft soft-deletes a draft (FR-20, FR-21). A registered or annulled
// invoice returns ErrNotDraft.
func (s *InvoiceService) DeleteDraft(ctx context.Context, actorID, condoID, id string) error {
	if err := s.Invoices.SoftDeleteDraft(ctx, condoID, id, actorID); err != nil {
		return err
	}
	recordAudit(ctx, s.Audit, actorID, model.AuditInvoiceDraftDeleted,
		map[string]any{"condominium_id": condoID, "invoice_id": id})
	return nil
}

// Annul annuls a registered invoice with a reason (FR-22).
func (s *InvoiceService) Annul(ctx context.Context, actorID, condoID, id, reason string) error {
	reason = strings.TrimSpace(reason)
	if err := s.Invoices.Annul(ctx, condoID, id, reason, actorID); err != nil {
		return err
	}
	recordAudit(ctx, s.Audit, actorID, model.AuditInvoiceAnnulled,
		map[string]any{"condominium_id": condoID, "invoice_id": id, "reason": reason})
	return nil
}

// AddPayment records a payment (FR-15, FR-17). A draft returns
// domain.ErrDraftNoPayments, an annulled invoice domain.ErrAnnulled, and an
// over-payment without justification domain.ErrOverpayment. The date defaults
// to today when zero.
func (s *InvoiceService) AddPayment(ctx context.Context, actorID, condoID, invoiceID string, p domain.Payment) (domain.Payment, error) {
	errs := domain.FieldErrors{}
	if !p.Method.Valid() {
		errs.Add("method", domain.KeyRequired)
	}
	if len(errs) > 0 {
		return domain.Payment{}, errs
	}
	if p.Date.IsZero() {
		p.Date = s.now()
	}
	p.CreatedBy = actorID

	// Early check against the current state for a clear error before the
	// locked write, which repeats it against the committed values.
	inv, err := s.Invoices.GetInvoice(ctx, condoID, invoiceID)
	if err != nil {
		return domain.Payment{}, err
	}
	if err := domain.CheckPayment(inv.State, inv.Totals.PayableCents, inv.PaidCents(), p.AmountCents, p.Justification); err != nil {
		return domain.Payment{}, err
	}

	out, err := s.Invoices.AddPayment(ctx, condoID, invoiceID, p)
	if err != nil {
		return domain.Payment{}, err
	}
	recordAudit(ctx, s.Audit, actorID, model.AuditInvoicePaymentAdded, map[string]any{
		"condominium_id": condoID, "invoice_id": invoiceID, "payment_id": out.ID, "amount_cents": out.AmountCents,
	})
	return out, nil
}

// AddAttachments stores the files and attaches them to the invoice (FR-25).
// The files are written before the repository takes the invoice lock and are
// removed again if it fails. At most 10 files per invoice; the repository
// enforces that under the lock.
func (s *InvoiceService) AddAttachments(ctx context.Context, actorID, condoID, invoiceID string, files []*multipart.FileHeader) ([]domain.Attachment, error) {
	if len(files) == 0 {
		return nil, ErrNoFiles
	}
	// Reject a foreign, annulled or full invoice before touching the disk.
	inv, err := s.Invoices.GetInvoice(ctx, condoID, invoiceID)
	if err != nil {
		return nil, err
	}
	if err := domain.CheckAttachmentChange(inv.State); err != nil {
		return nil, err
	}
	if err := domain.CheckAttachmentCount(len(inv.Attachments), len(files)); err != nil {
		return nil, err
	}

	dir := s.attachmentsDir()
	var stored []uploads.Stored
	cleanup := func() {
		for _, st := range stored {
			if err := uploads.Remove(dir, st.Path); err != nil {
				slog.Error("invoices: remove orphan attachment", "path", st.Path, "error", err)
			}
		}
	}
	atts := make([]domain.Attachment, 0, len(files))
	for _, fh := range files {
		st, err := uploads.Save(dir, fh, domain.MaxAttachmentBytes)
		if err != nil {
			cleanup()
			if errors.Is(err, uploads.ErrInvalid) {
				return nil, ErrInvalidFile
			}
			return nil, err
		}
		stored = append(stored, st)
		atts = append(atts, domain.Attachment{
			InvoiceID: invoiceID, OriginalName: st.OriginalName, StoredPath: st.Path,
			ContentType: st.ContentType, SizeBytes: st.Size, CreatedBy: actorID,
		})
	}

	saved, err := s.Invoices.AddAttachments(ctx, condoID, invoiceID, atts)
	if err != nil {
		cleanup()
		return nil, err
	}
	for _, a := range saved {
		recordAudit(ctx, s.Audit, actorID, model.AuditInvoiceAttachmentAdded, map[string]any{
			"condominium_id": condoID, "invoice_id": invoiceID, "attachment_id": a.ID,
		})
	}
	return saved, nil
}

// RemoveAttachment deletes an attachment row under the invoice lock and, once
// that commit succeeded, its stored file.
func (s *InvoiceService) RemoveAttachment(ctx context.Context, actorID, condoID, invoiceID, attID string) error {
	a, err := s.Invoices.RemoveAttachment(ctx, condoID, invoiceID, attID)
	if err != nil {
		return err
	}
	if a.StoredPath != "" {
		if err := uploads.Remove(s.attachmentsDir(), a.StoredPath); err != nil {
			slog.Error("invoices: remove attachment file", "path", a.StoredPath, "error", err)
		}
	}
	recordAudit(ctx, s.Audit, actorID, model.AuditInvoiceAttachmentRemoved, map[string]any{
		"condominium_id": condoID, "invoice_id": invoiceID, "attachment_id": attID,
	})
	return nil
}

// OpenAttachment opens an attachment for download (FR-26). It first checks
// that the attachment belongs to the invoice of the condominium; any other
// condominium, invoice or traversal attempt is ErrNotFound. The caller closes
// the file.
func (s *InvoiceService) OpenAttachment(ctx context.Context, condoID, invoiceID, attID string) (*os.File, domain.Attachment, error) {
	a, err := s.Invoices.GetAttachment(ctx, condoID, invoiceID, attID)
	if err != nil {
		return nil, domain.Attachment{}, err
	}
	f, err := uploads.Open(s.attachmentsDir(), a.StoredPath)
	if err != nil {
		if errors.Is(err, uploads.ErrInvalid) || errors.Is(err, os.ErrNotExist) {
			return nil, domain.Attachment{}, ErrNotFound
		}
		return nil, domain.Attachment{}, err
	}
	return f, a, nil
}

// Get returns an invoice of the condominium with its lines, payments and
// attachments, or ErrNotFound.
func (s *InvoiceService) Get(ctx context.Context, condoID, id string) (domain.Invoice, error) {
	return s.Invoices.GetInvoice(ctx, condoID, id)
}

// List returns the invoices of the condominium matching the filter, with the
// totals of the whole filtered set.
func (s *InvoiceService) List(ctx context.Context, condoID string, f domain.ListFilter) (domain.ListResult, error) {
	return s.Invoices.List(ctx, condoID, f)
}
