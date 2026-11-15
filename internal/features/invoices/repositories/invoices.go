package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
)

// InvoiceStore persists invoices with their lines, VAT breakdown, payments and
// attachments.
//
// Every write to an existing invoice goes through WithLockedInvoice: the row is
// locked with SELECT ... FOR UPDATE and the domain check runs inside the same
// transaction against committed values, so edits, payments, annulments and
// attachment changes cannot race.
type InvoiceStore struct {
	db *sql.DB
}

// NewInvoiceStore creates an InvoiceStore.
func NewInvoiceStore(db *sql.DB) *InvoiceStore { return &InvoiceStore{db: db} }

// invoiceCols are the editable header columns, in the order of invoiceValues.
var invoiceCols = []string{
	"document_type", "expense_type", "number", "issue_date", "due_date", "operation_date",
	"supplier_id", "description", "due_justification", "category", "project", "cost_center",
	"reference_period", "notes", "fiscal_region",
	"service_description", "service_start_date", "service_end_date", "service_contract",
	"monthly_service_type", "monthly_billing_period", "monthly_start_date", "monthly_end_date",
	"monthly_contract_number", "monthly_meter_id", "monthly_cpe", "monthly_cui",
	"monthly_consumption_place", "monthly_recurring",
	"original_invoice_id",
	"gross_cents", "discount_cents", "base_cents", "vat_cents", "total_cents",
	"withholding_cents", "payable_cents",
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullDate(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func invoiceValues(inv domain.Invoice) []any {
	region := string(inv.FiscalRegion)
	if region == "" {
		region = string(domain.RegionContinental)
	}
	t := inv.Totals
	return []any{
		nullStr(string(inv.DocumentType)), nullStr(string(inv.ExpenseType)),
		nullStr(domain.NormalizeNumber(inv.Number)),
		nullDate(inv.IssueDate), nullDate(inv.DueDate), nullDate(inv.OperationDate),
		nullStr(inv.SupplierID), nullStr(inv.Description), inv.DueJustification,
		nullStr(string(inv.Category)), inv.Project, inv.CostCenter,
		nullStr(inv.ReferencePeriod), inv.Notes, region,
		inv.Service.Description, nullDate(inv.Service.StartDate), nullDate(inv.Service.EndDate), inv.Service.Contract,
		inv.Monthly.ServiceType, inv.Monthly.BillingPeriod, nullDate(inv.Monthly.StartDate), nullDate(inv.Monthly.EndDate),
		inv.Monthly.ContractNumber, inv.Monthly.MeterID, inv.Monthly.CPE, inv.Monthly.CUI,
		inv.Monthly.ConsumptionPlace, inv.Monthly.Recurring,
		nullStr(inv.OriginalInvoiceID),
		t.GrossCents, t.DiscountCents, t.BaseCents, t.VATCents, t.TotalCents,
		t.WithholdingCents, t.PayableCents,
	}
}

func init() {
	if n, m := len(invoiceCols), len(invoiceValues(domain.Invoice{})); n != m {
		panic(fmt.Sprintf("invoices repository: %d columns but %d values", n, m))
	}
}

func actorOf(inv domain.Invoice) string {
	if inv.UpdatedBy != "" {
		return inv.UpdatedBy
	}
	return inv.CreatedBy
}

// execer is satisfied by *sql.Tx.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// replaceChildren rewrites the lines and the VAT breakdown of an invoice.
func replaceChildren(ctx context.Context, tx execer, condoID, id, actor string, inv domain.Invoice) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM invoice_items WHERE condominium_id = $1 AND invoice_id = $2`, condoID, id); err != nil {
		return fmt.Errorf("delete lines: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM invoice_taxes WHERE condominium_id = $1 AND invoice_id = $2`, condoID, id); err != nil {
		return fmt.Errorf("delete taxes: %w", err)
	}
	for i, l := range inv.Lines {
		base := l.BaseCents
		if b, err := domain.LineBase(l); err == nil {
			base = b
		}
		treatment := l.Treatment
		if treatment == "" {
			treatment = domain.TreatmentRate
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO invoice_items (condominium_id, invoice_id, position, description,
				product, reference, unit, quantity_milli, unit_price_cents, discount_cents,
				base_cents, treatment, rate_bp, exemption_code, exemption_reason,
				created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $16)`,
			condoID, id, i, l.Description, l.Product, l.Reference, l.Unit, l.QuantityMilli,
			l.UnitPriceCents, l.DiscountCents, base, string(treatment), l.RateBP,
			l.ExemptionCode, l.ExemptionReason, actor); err != nil {
			return fmt.Errorf("insert line %d: %w", i, err)
		}
	}
	for i, b := range inv.Totals.Taxes {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO invoice_taxes (condominium_id, invoice_id, position, treatment,
				rate_bp, base_cents, vat_cents, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`,
			condoID, id, i, string(b.Treatment), b.RateBP, b.BaseCents, b.VATCents, actor); err != nil {
			return fmt.Errorf("insert tax %d: %w", i, err)
		}
	}
	return nil
}

// mapErr turns driver errors into typed repository errors. It must run after
// the failed transaction was rolled back, because it may query for the id of
// the conflicting invoice.
func (r *InvoiceStore) mapErr(ctx context.Context, inv domain.Invoice, err error) error {
	pe := pgError(err)
	if pe == nil {
		return err
	}
	switch pe.Code {
	case "23505":
		switch pe.ConstraintName {
		case constraintDuplicate:
			existing, ferr := r.FindDuplicate(ctx, inv.CondominiumID, inv.SupplierID,
				inv.DocumentType, inv.Number, inv.ID)
			if ferr != nil && !errors.Is(ferr, ErrNotFound) {
				return ferr
			}
			return &DuplicateInvoiceError{ExistingID: existing}
		case constraintIdempotency:
			existing, ferr := r.FindByIdempotencyKey(ctx, inv.CondominiumID, inv.IdempotencyKey)
			if ferr != nil && !errors.Is(ferr, ErrNotFound) {
				return ferr
			}
			return &DuplicateKeyError{ExistingID: existing}
		}
	case "23503":
		return fmt.Errorf("%w: %s", ErrInvalidReference, pe.ConstraintName)
	case "22P02":
		return ErrNotFound
	}
	return err
}

// SaveInvoice inserts a new draft or registered invoice with its lines and VAT
// breakdown in one transaction and returns its id. inv.CondominiumID,
// inv.CreatedBy and the totals (from domain.ComputeTotals) must be set.
//
// Errors: *DuplicateInvoiceError (FR-23), *DuplicateKeyError (FR-24) and
// ErrInvalidReference (e.g. a supplier of another condominium).
func (r *InvoiceStore) SaveInvoice(ctx context.Context, inv domain.Invoice) (string, error) {
	if inv.State != domain.StateDraft && inv.State != domain.StateRegistered {
		return "", ErrInvalidTransition
	}
	actor := inv.CreatedBy
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin save invoice: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	placeholders := make([]string, len(invoiceCols))
	for i := range invoiceCols {
		placeholders[i] = "$" + strconv.Itoa(i+5)
	}
	args := append([]any{inv.CondominiumID, string(inv.State), nullStr(inv.IdempotencyKey), actor},
		invoiceValues(inv)...)
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO invoices (condominium_id, state, idempotency_key, created_by, updated_by, `+
		strings.Join(invoiceCols, ", ")+`)
		VALUES ($1, $2, $3, $4, $4, `+strings.Join(placeholders, ", ")+`)
		RETURNING id`, args...).Scan(&id)
	if err == nil {
		err = replaceChildren(ctx, tx, inv.CondominiumID, id, actor, inv)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		_ = tx.Rollback()
		return "", fmt.Errorf("save invoice: %w", r.mapErr(ctx, inv, err))
	}
	return id, nil
}

// FindByIdempotencyKey returns the id of the invoice created with the key in
// the condominium, or ErrNotFound.
func (r *InvoiceStore) FindByIdempotencyKey(ctx context.Context, condoID, key string) (string, error) {
	if key == "" {
		return "", ErrNotFound
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		SELECT id FROM invoices
		WHERE condominium_id = $1 AND idempotency_key = $2 AND deleted_at IS NULL`,
		condoID, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find by idempotency key: %w", err)
	}
	return id, nil
}

// FindDuplicate returns the id of the invoice that holds the document key
// (supplier, document type, number), ignoring excludeID, soft-deleted drafts
// excluded and annulled invoices included (FR-23). ErrNotFound when free.
func (r *InvoiceStore) FindDuplicate(ctx context.Context, condoID, supplierID string,
	docType domain.DocumentType, number, excludeID string) (string, error) {
	number = domain.NormalizeNumber(number)
	if supplierID == "" || number == "" {
		return "", ErrNotFound
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		SELECT id FROM invoices
		WHERE condominium_id = $1 AND supplier_id = $2 AND document_type = $3
			AND number = $4 AND deleted_at IS NULL
			AND ($5 = '' OR id::text <> $5)
		LIMIT 1`,
		condoID, supplierID, string(docType), number, excludeID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find duplicate invoice: %w", mapNotFoundPg(err))
	}
	return id, nil
}

func mapNotFoundPg(err error) error {
	if pe := pgError(err); pe != nil && pe.Code == "22P02" {
		return ErrNotFound
	}
	return err
}

// GetInvoice returns a non-deleted invoice of the condominium with its
// supplier, lines, VAT breakdown, payments and attachments, or ErrNotFound.
func (r *InvoiceStore) GetInvoice(ctx context.Context, condoID, id string) (domain.Invoice, error) {
	inv, err := r.loadHeader(ctx, condoID, id)
	if err != nil {
		return domain.Invoice{}, err
	}
	if inv.SupplierID != "" {
		sup, err := scanSupplier(r.db.QueryRowContext(ctx,
			`SELECT `+supplierCols+` FROM suppliers WHERE condominium_id = $1 AND id = $2`,
			condoID, inv.SupplierID))
		if err != nil {
			return domain.Invoice{}, fmt.Errorf("load invoice supplier: %w", err)
		}
		inv.Supplier = &sup
	}
	if err := r.loadChildren(ctx, &inv); err != nil {
		return domain.Invoice{}, err
	}
	return inv, nil
}

func dateOf(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	t := n.Time
	return &t
}

func (r *InvoiceStore) loadHeader(ctx context.Context, condoID, id string) (domain.Invoice, error) {
	var (
		inv                                                       domain.Invoice
		idemKey, docType, expType, number, desc, category, refPer sql.NullString
		supplierID, originalID                                    sql.NullString
		issue, due, operation, svcStart, svcEnd, moStart, moEnd   sql.NullTime
		annulledAt, deletedAt                                     sql.NullTime
		state, region                                             string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, condominium_id, state, idempotency_key,
			document_type, expense_type, number, issue_date, due_date, operation_date,
			supplier_id, description, due_justification, category, project, cost_center,
			reference_period, notes, fiscal_region,
			service_description, service_start_date, service_end_date, service_contract,
			monthly_service_type, monthly_billing_period, monthly_start_date, monthly_end_date,
			monthly_contract_number, monthly_meter_id, monthly_cpe, monthly_cui,
			monthly_consumption_place, monthly_recurring, original_invoice_id,
			gross_cents, discount_cents, base_cents, vat_cents, total_cents,
			withholding_cents, payable_cents,
			annul_reason, annulled_at, deleted_at,
			created_at, updated_at, created_by, updated_by
		FROM invoices
		WHERE condominium_id = $1 AND id = $2 AND deleted_at IS NULL`, condoID, id).Scan(
		&inv.ID, &inv.CondominiumID, &state, &idemKey,
		&docType, &expType, &number, &issue, &due, &operation,
		&supplierID, &desc, &inv.DueJustification, &category, &inv.Project, &inv.CostCenter,
		&refPer, &inv.Notes, &region,
		&inv.Service.Description, &svcStart, &svcEnd, &inv.Service.Contract,
		&inv.Monthly.ServiceType, &inv.Monthly.BillingPeriod, &moStart, &moEnd,
		&inv.Monthly.ContractNumber, &inv.Monthly.MeterID, &inv.Monthly.CPE, &inv.Monthly.CUI,
		&inv.Monthly.ConsumptionPlace, &inv.Monthly.Recurring, &originalID,
		&inv.Totals.GrossCents, &inv.Totals.DiscountCents, &inv.Totals.BaseCents,
		&inv.Totals.VATCents, &inv.Totals.TotalCents,
		&inv.Totals.WithholdingCents, &inv.Totals.PayableCents,
		&inv.AnnulReason, &annulledAt, &deletedAt,
		&inv.CreatedAt, &inv.UpdatedAt, &inv.CreatedBy, &inv.UpdatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Invoice{}, ErrNotFound
	}
	if err != nil {
		return domain.Invoice{}, fmt.Errorf("get invoice: %w", mapNotFoundPg(err))
	}
	inv.State = domain.State(state)
	inv.FiscalRegion = domain.FiscalRegion(region)
	inv.IdempotencyKey = idemKey.String
	inv.DocumentType = domain.DocumentType(docType.String)
	inv.ExpenseType = domain.ExpenseType(expType.String)
	inv.Number = number.String
	inv.IssueDate, inv.DueDate, inv.OperationDate = dateOf(issue), dateOf(due), dateOf(operation)
	inv.SupplierID = supplierID.String
	inv.Description = desc.String
	inv.Category = domain.Category(category.String)
	inv.ReferencePeriod = refPer.String
	inv.Service.StartDate, inv.Service.EndDate = dateOf(svcStart), dateOf(svcEnd)
	inv.Monthly.StartDate, inv.Monthly.EndDate = dateOf(moStart), dateOf(moEnd)
	inv.OriginalInvoiceID = originalID.String
	inv.AnnulledAt, inv.DeletedAt = dateOf(annulledAt), dateOf(deletedAt)
	return inv, nil
}

func (r *InvoiceStore) loadChildren(ctx context.Context, inv *domain.Invoice) error {
	condo, id := inv.CondominiumID, inv.ID

	rows, err := r.db.QueryContext(ctx, `
		SELECT description, product, reference, unit, quantity_milli, unit_price_cents,
			discount_cents, base_cents, treatment, rate_bp, exemption_code, exemption_reason
		FROM invoice_items WHERE condominium_id = $1 AND invoice_id = $2 ORDER BY position`, condo, id)
	if err != nil {
		return fmt.Errorf("load lines: %w", err)
	}
	for rows.Next() {
		var l domain.Line
		var tr string
		if err := rows.Scan(&l.Description, &l.Product, &l.Reference, &l.Unit, &l.QuantityMilli,
			&l.UnitPriceCents, &l.DiscountCents, &l.BaseCents, &tr, &l.RateBP,
			&l.ExemptionCode, &l.ExemptionReason); err != nil {
			rows.Close()
			return fmt.Errorf("scan line: %w", err)
		}
		l.Treatment = domain.VATTreatment(tr)
		inv.Lines = append(inv.Lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("load lines: %w", err)
	}

	rows, err = r.db.QueryContext(ctx, `
		SELECT treatment, rate_bp, base_cents, vat_cents
		FROM invoice_taxes WHERE condominium_id = $1 AND invoice_id = $2 ORDER BY position`, condo, id)
	if err != nil {
		return fmt.Errorf("load taxes: %w", err)
	}
	for rows.Next() {
		var b domain.TaxBucket
		var tr string
		if err := rows.Scan(&tr, &b.RateBP, &b.BaseCents, &b.VATCents); err != nil {
			rows.Close()
			return fmt.Errorf("scan tax: %w", err)
		}
		b.Treatment = domain.VATTreatment(tr)
		inv.Totals.Taxes = append(inv.Totals.Taxes, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("load taxes: %w", err)
	}

	rows, err = r.db.QueryContext(ctx, `
		SELECT id, payment_date, amount_cents, method, reference, bank_account,
			justification, created_at, created_by
		FROM invoice_payments WHERE condominium_id = $1 AND invoice_id = $2
		ORDER BY payment_date, created_at, id`, condo, id)
	if err != nil {
		return fmt.Errorf("load payments: %w", err)
	}
	for rows.Next() {
		p := domain.Payment{InvoiceID: id}
		var m string
		if err := rows.Scan(&p.ID, &p.Date, &p.AmountCents, &m, &p.Reference, &p.BankAccount,
			&p.Justification, &p.CreatedAt, &p.CreatedBy); err != nil {
			rows.Close()
			return fmt.Errorf("scan payment: %w", err)
		}
		p.Method = domain.PaymentMethod(m)
		inv.Payments = append(inv.Payments, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("load payments: %w", err)
	}

	rows, err = r.db.QueryContext(ctx, `
		SELECT id, original_name, stored_path, content_type, size_bytes, created_at, created_by
		FROM invoice_attachments WHERE condominium_id = $1 AND invoice_id = $2
		ORDER BY created_at, id`, condo, id)
	if err != nil {
		return fmt.Errorf("load attachments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		a := domain.Attachment{InvoiceID: id}
		if err := rows.Scan(&a.ID, &a.OriginalName, &a.StoredPath, &a.ContentType,
			&a.SizeBytes, &a.CreatedAt, &a.CreatedBy); err != nil {
			return fmt.Errorf("scan attachment: %w", err)
		}
		inv.Attachments = append(inv.Attachments, a)
	}
	return rows.Err()
}

// GetAttachment returns one attachment of a non-deleted invoice of the
// condominium, or ErrNotFound.
func (r *InvoiceStore) GetAttachment(ctx context.Context, condoID, invoiceID, attID string) (domain.Attachment, error) {
	a := domain.Attachment{InvoiceID: invoiceID}
	err := r.db.QueryRowContext(ctx, `
		SELECT a.id, a.original_name, a.stored_path, a.content_type, a.size_bytes,
			a.created_at, a.created_by
		FROM invoice_attachments a
		JOIN invoices i ON i.condominium_id = a.condominium_id AND i.id = a.invoice_id
		WHERE a.condominium_id = $1 AND a.invoice_id = $2 AND a.id = $3
			AND i.deleted_at IS NULL`, condoID, invoiceID, attID).Scan(
		&a.ID, &a.OriginalName, &a.StoredPath, &a.ContentType, &a.SizeBytes, &a.CreatedAt, &a.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Attachment{}, ErrNotFound
	}
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("get attachment: %w", mapNotFoundPg(err))
	}
	return a, nil
}
