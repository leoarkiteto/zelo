package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
)

// LockedInvoice is the committed state of an invoice whose row is locked
// FOR UPDATE inside a WithLockedInvoice transaction. The values are read after
// the lock was granted, so they cannot change until the callback returns.
//
// Its methods are the only write helpers available to a callback; they keep
// the fields below in sync with what they write.
type LockedInvoice struct {
	ID              string
	CondominiumID   string
	State           domain.State
	PayableCents    int64
	PaidCents       int64
	AttachmentCount int

	tx *sql.Tx
}

// WithLockedInvoice runs fn in a transaction that holds a row lock on the
// (non-deleted) invoice of the condominium. fn runs the domain checks against
// the locked values and then writes through the LockedInvoice helpers. A
// non-nil error from fn rolls the transaction back and is returned as is.
// ErrNotFound when the invoice does not exist in the condominium.
func (r *InvoiceStore) WithLockedInvoice(ctx context.Context, condoID, id string,
	fn func(ctx context.Context, l *LockedInvoice) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin locked invoice: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	l := &LockedInvoice{ID: id, CondominiumID: condoID, tx: tx}
	var state string
	err = tx.QueryRowContext(ctx, `
		SELECT state, payable_cents FROM invoices
		WHERE condominium_id = $1 AND id = $2 AND deleted_at IS NULL
		FOR UPDATE`, condoID, id).Scan(&state, &l.PayableCents)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock invoice: %w", mapNotFoundPg(err))
	}
	l.State = domain.State(state)

	// Separate statements: after waiting for the lock, a new statement sees
	// everything committed by the previous holder, while a sub-select of the
	// locking statement would use the older snapshot.
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0) FROM invoice_payments
		WHERE condominium_id = $1 AND invoice_id = $2`, condoID, id).Scan(&l.PaidCents); err != nil {
		return fmt.Errorf("sum payments: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT count(*) FROM invoice_attachments
		WHERE condominium_id = $1 AND invoice_id = $2`, condoID, id).Scan(&l.AttachmentCount); err != nil {
		return fmt.Errorf("count attachments: %w", err)
	}

	if err := fn(ctx, l); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit locked invoice: %w", err)
	}
	return nil
}

// UpdateInvoice replaces the header, lines and VAT breakdown of the locked
// invoice. inv.State is the target state: draft to draft, draft to registered
// and registered to registered are allowed; anything else returns
// ErrInvalidTransition. The idempotency key and the created_* columns are not
// touched. The caller runs the domain checks first.
func (l *LockedInvoice) UpdateInvoice(ctx context.Context, inv domain.Invoice) error {
	if inv.State != domain.StateDraft && inv.State != domain.StateRegistered {
		return ErrInvalidTransition
	}
	if l.State == domain.StateRegistered && inv.State != domain.StateRegistered {
		return ErrInvalidTransition
	}
	actor := actorOf(inv)
	sets := make([]string, len(invoiceCols))
	for i, c := range invoiceCols {
		sets[i] = c + " = $" + strconv.Itoa(i+5)
	}
	args := append([]any{l.CondominiumID, l.ID, string(inv.State), actor}, invoiceValues(inv)...)
	if _, err := l.tx.ExecContext(ctx, `
		UPDATE invoices SET state = $3, updated_by = $4, updated_at = now(), `+
		strings.Join(sets, ", ")+`
		WHERE condominium_id = $1 AND id = $2`, args...); err != nil {
		return fmt.Errorf("update invoice: %w", err)
	}
	if err := replaceChildren(ctx, l.tx, l.CondominiumID, l.ID, actor, inv); err != nil {
		return err
	}
	l.State = inv.State
	l.PayableCents = inv.Totals.PayableCents
	return nil
}

// AddPayment inserts a payment and returns it with its id.
func (l *LockedInvoice) AddPayment(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	err := l.tx.QueryRowContext(ctx, `
		INSERT INTO invoice_payments (condominium_id, invoice_id, payment_date, amount_cents,
			method, reference, bank_account, justification, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		RETURNING id, created_at`,
		l.CondominiumID, l.ID, p.Date, p.AmountCents, string(p.Method), p.Reference,
		p.BankAccount, p.Justification, p.CreatedBy).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		return domain.Payment{}, fmt.Errorf("insert payment: %w", err)
	}
	p.InvoiceID = l.ID
	l.PaidCents += p.AmountCents
	return p, nil
}

// MarkAnnulled sets the state to annulled with the reason.
func (l *LockedInvoice) MarkAnnulled(ctx context.Context, reason, actor string) error {
	if _, err := l.tx.ExecContext(ctx, `
		UPDATE invoices SET state = 'annulled', annulled_at = now(), annul_reason = $3,
			updated_by = $4, updated_at = now()
		WHERE condominium_id = $1 AND id = $2`, l.CondominiumID, l.ID, reason, actor); err != nil {
		return fmt.Errorf("annul invoice: %w", err)
	}
	l.State = domain.StateAnnulled
	return nil
}

// MarkDeleted soft-deletes the draft and releases its idempotency key.
func (l *LockedInvoice) MarkDeleted(ctx context.Context, actor string) error {
	if _, err := l.tx.ExecContext(ctx, `
		UPDATE invoices SET deleted_at = now(), idempotency_key = NULL,
			updated_by = $3, updated_at = now()
		WHERE condominium_id = $1 AND id = $2`, l.CondominiumID, l.ID, actor); err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}
	return nil
}

// AddAttachment inserts an attachment row and returns it with its id.
func (l *LockedInvoice) AddAttachment(ctx context.Context, a domain.Attachment) (domain.Attachment, error) {
	err := l.tx.QueryRowContext(ctx, `
		INSERT INTO invoice_attachments (condominium_id, invoice_id, stored_path, original_name,
			content_type, size_bytes, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING id, created_at`,
		l.CondominiumID, l.ID, a.StoredPath, a.OriginalName, a.ContentType, a.SizeBytes,
		a.CreatedBy).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("insert attachment: %w", err)
	}
	a.InvoiceID = l.ID
	l.AttachmentCount++
	return a, nil
}

// DeleteAttachment removes an attachment row and returns it so the caller can
// remove the stored file after the commit. ErrNotFound when it is not there.
func (l *LockedInvoice) DeleteAttachment(ctx context.Context, attID string) (domain.Attachment, error) {
	a := domain.Attachment{InvoiceID: l.ID}
	err := l.tx.QueryRowContext(ctx, `
		DELETE FROM invoice_attachments
		WHERE condominium_id = $1 AND invoice_id = $2 AND id = $3
		RETURNING id, original_name, stored_path, content_type, size_bytes, created_at, created_by`,
		l.CondominiumID, l.ID, attID).Scan(&a.ID, &a.OriginalName, &a.StoredPath,
		&a.ContentType, &a.SizeBytes, &a.CreatedAt, &a.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Attachment{}, ErrNotFound
	}
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("delete attachment: %w", mapNotFoundPg(err))
	}
	l.AttachmentCount--
	return a, nil
}

// UpdateInvoice saves an edit of the invoice inv.ID of inv.CondominiumID under
// the row lock. Inside the transaction it applies domain.CheckEditable (an
// annulled invoice is rejected with domain.ErrAnnulled) and domain.CheckEdit
// against the committed paid sum (domain.ErrPayableBelowPaid, FR-19), then
// replaces header, lines and VAT breakdown. Draft to registered completes the
// draft. Duplicate and reference errors are typed as in SaveInvoice.
func (r *InvoiceStore) UpdateInvoice(ctx context.Context, inv domain.Invoice) error {
	err := r.WithLockedInvoice(ctx, inv.CondominiumID, inv.ID, func(ctx context.Context, l *LockedInvoice) error {
		if err := domain.CheckEditable(l.State); err != nil {
			return err
		}
		if err := domain.CheckEdit(inv.Totals.PayableCents, l.PaidCents); err != nil {
			return err
		}
		return l.UpdateInvoice(ctx, inv)
	})
	if err != nil {
		return r.mapErr(ctx, inv, err)
	}
	return nil
}

// AddPayment records a payment under the row lock, applying domain.CheckPayment
// to the committed state, payable and paid values (draft, annulled, amount and
// over-payment rules; FR-15, FR-17, FR-20, FR-22). It returns the stored
// payment.
func (r *InvoiceStore) AddPayment(ctx context.Context, condoID, invoiceID string, p domain.Payment) (domain.Payment, error) {
	var out domain.Payment
	err := r.WithLockedInvoice(ctx, condoID, invoiceID, func(ctx context.Context, l *LockedInvoice) error {
		if err := domain.CheckPayment(l.State, l.PayableCents, l.PaidCents, p.AmountCents, p.Justification); err != nil {
			return err
		}
		var err error
		out, err = l.AddPayment(ctx, p)
		return err
	})
	return out, err
}

// Annul annuls a registered invoice under the row lock (domain.CheckAnnul).
// Lines, payments and attachments are kept (FR-22).
func (r *InvoiceStore) Annul(ctx context.Context, condoID, invoiceID, reason, actor string) error {
	return r.WithLockedInvoice(ctx, condoID, invoiceID, func(ctx context.Context, l *LockedInvoice) error {
		if err := domain.CheckAnnul(l.State, reason); err != nil {
			return err
		}
		return l.MarkAnnulled(ctx, strings.TrimSpace(reason), actor)
	})
}

// SoftDeleteDraft soft-deletes a draft under the row lock. ErrNotDraft when
// the invoice is registered or annulled.
func (r *InvoiceStore) SoftDeleteDraft(ctx context.Context, condoID, invoiceID, actor string) error {
	return r.WithLockedInvoice(ctx, condoID, invoiceID, func(ctx context.Context, l *LockedInvoice) error {
		if l.State != domain.StateDraft {
			return ErrNotDraft
		}
		return l.MarkDeleted(ctx, actor)
	})
}

// AddAttachments inserts attachment rows under the row lock, applying
// domain.CheckAttachmentChange and domain.CheckAttachmentCount to the
// committed count so the 10-file limit is atomic (FR-25).
func (r *InvoiceStore) AddAttachments(ctx context.Context, condoID, invoiceID string, atts []domain.Attachment) ([]domain.Attachment, error) {
	out := make([]domain.Attachment, 0, len(atts))
	err := r.WithLockedInvoice(ctx, condoID, invoiceID, func(ctx context.Context, l *LockedInvoice) error {
		if err := domain.CheckAttachmentChange(l.State); err != nil {
			return err
		}
		if err := domain.CheckAttachmentCount(l.AttachmentCount, len(atts)); err != nil {
			return err
		}
		for _, a := range atts {
			saved, err := l.AddAttachment(ctx, a)
			if err != nil {
				return err
			}
			out = append(out, saved)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveAttachment deletes an attachment row under the row lock
// (domain.CheckAttachmentChange) and returns it, so the caller can delete the
// stored file after the commit.
func (r *InvoiceStore) RemoveAttachment(ctx context.Context, condoID, invoiceID, attID string) (domain.Attachment, error) {
	var out domain.Attachment
	err := r.WithLockedInvoice(ctx, condoID, invoiceID, func(ctx context.Context, l *LockedInvoice) error {
		if err := domain.CheckAttachmentChange(l.State); err != nil {
			return err
		}
		var err error
		out, err = l.DeleteAttachment(ctx, attID)
		return err
	})
	return out, err
}
