// Package repositories implements the finance persistence adapters using
// database/sql and plain SQL (constitution VI).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/pagination"
	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
)

// AccountStore persists financial accounts.
type AccountStore struct {
	db *sql.DB
}

// NewAccountStore creates an AccountStore.
func NewAccountStore(db *sql.DB) *AccountStore { return &AccountStore{db: db} }

// Create inserts an account and returns its id.
func (s *AccountStore) Create(ctx context.Context, a domain.FinancialAccount) (string, error) {
	id, err := insertAccount(ctx, s.db, a)
	if err != nil {
		return "", fmt.Errorf("create account: %w", err)
	}
	return id, nil
}

// dbtx is satisfied by *sql.DB and *sql.Tx.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// insertAccount is the single finance_accounts INSERT, shared by
// AccountStore.Create and the quota issuance (QuotaStore InsertCharges) so the
// two cannot drift. It returns the new id.
func insertAccount(ctx context.Context, q dbtx, a domain.FinancialAccount) (string, error) {
	var id string
	err := q.QueryRowContext(ctx, `
		INSERT INTO finance_accounts
			(condominium_id, account_type, title, category, amount_cents, due_date,
			 status, settlement_date, supplier_payee, receipt_path, receipt_name,
			 unit_id, payment_code, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id`,
		a.CondominiumID, a.Type, a.Title, a.Category, a.AmountCents, a.DueDate,
		a.Status, nullTime(a.SettlementDate), sharedstore.NullString(a.SupplierPayee),
		sharedstore.NullString(a.ReceiptPath), sharedstore.NullString(a.ReceiptName),
		sharedstore.NullString(a.UnitID), sharedstore.NullString(a.PaymentCode), a.CreatedBy).Scan(&id)
	return id, err
}

// GetByID returns an account by id.
func (s *AccountStore) GetByID(ctx context.Context, id string) (domain.FinancialAccount, error) {
	var a domain.FinancialAccount
	var settlementDate sql.NullTime
	var supplier, receiptPath, receiptName, unitID, paymentCode sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, condominium_id, account_type, title, category, amount_cents,
			due_date, status, settlement_date, supplier_payee, receipt_path,
			receipt_name, unit_id, payment_code, created_by, created_at, updated_at
		FROM finance_accounts WHERE id = $1`, id).Scan(
		&a.ID, &a.CondominiumID, &a.Type, &a.Title, &a.Category, &a.AmountCents,
		&a.DueDate, &a.Status, &settlementDate, &supplier, &receiptPath, &receiptName,
		&unitID, &paymentCode, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.FinancialAccount{}, sharedstore.ErrNotFound
	}
	if err != nil {
		return domain.FinancialAccount{}, fmt.Errorf("get account: %w", err)
	}
	if settlementDate.Valid {
		a.SettlementDate = &settlementDate.Time
	}
	a.SupplierPayee = supplier.String
	a.ReceiptPath = receiptPath.String
	a.ReceiptName = receiptName.String
	a.UnitID = unitID.String
	a.PaymentCode = paymentCode.String
	return a, nil
}

// Update replaces the editable fields of an account. Only pending accounts
// (which includes derived overdue) are editable (FR-019); the row-level guard
// makes the transition contract enforceable even if the service is bypassed.
func (s *AccountStore) Update(ctx context.Context, a domain.FinancialAccount) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE finance_accounts
		SET title = $3, category = $4, amount_cents = $5, due_date = $6,
			supplier_payee = $7, receipt_path = $8, receipt_name = $9,
			unit_id = $10, payment_code = $11, updated_at = now()
		WHERE id = $1 AND condominium_id = $2 AND status IN ('pending')`,
		a.ID, a.CondominiumID, a.Title, a.Category, a.AmountCents, a.DueDate,
		sharedstore.NullString(a.SupplierPayee), sharedstore.NullString(a.ReceiptPath),
		sharedstore.NullString(a.ReceiptName), sharedstore.NullString(a.UnitID),
		sharedstore.NullString(a.PaymentCode))
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sharedstore.ErrNotFound
	}
	return nil
}

// SetStatus transitions an account's status. Only pending accounts can move
// (to settled or canceled); settled/canceled are terminal (FR-019).
func (s *AccountStore) SetStatus(ctx context.Context, id, condominiumID string, status domain.AccountStatus, settlementDate *time.Time) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE finance_accounts
		SET status = $3, settlement_date = $4, updated_at = now()
		WHERE id = $1 AND condominium_id = $2 AND status IN ('pending')`,
		id, condominiumID, status, nullTime(settlementDate))
	if err != nil {
		return fmt.Errorf("set account status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sharedstore.ErrNotFound
	}
	return nil
}

// List returns accounts of a condominium matching the filter.
func (s *AccountStore) List(ctx context.Context, condominiumID string, f services.AccountFilter) ([]domain.FinancialAccount, error) {
	whereSQL, params := accountWhere(condominiumID, f)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+accountColumns+`
		FROM finance_accounts
		WHERE `+whereSQL+`
		ORDER BY due_date, title`, params...)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()
	return scanAccounts(rows)
}

// ListPage returns one page (pagination.PageSize rows) of the accounts matching
// the filter, in the same order as List with id as a stable tie-breaker. The
// requested page is clamped to the existing range.
func (s *AccountStore) ListPage(ctx context.Context, condominiumID string, f services.AccountFilter, requested int) ([]domain.FinancialAccount, pagination.Page, error) {
	whereSQL, params := accountWhere(condominiumID, f)
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM finance_accounts WHERE `+whereSQL, params...).Scan(&total); err != nil {
		return nil, pagination.Page{}, fmt.Errorf("count accounts: %w", err)
	}
	p := pagination.New(requested, total)
	if total == 0 {
		return nil, p, nil
	}
	n := len(params)
	paged := append(append([]any{}, params...), p.Size, p.Offset())
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+accountColumns+`
		FROM finance_accounts
		WHERE `+whereSQL+`
		ORDER BY due_date, title, id
		LIMIT $`+strconv.Itoa(n+1)+` OFFSET $`+strconv.Itoa(n+2), paged...)
	if err != nil {
		return nil, pagination.Page{}, fmt.Errorf("list accounts page: %w", err)
	}
	defer rows.Close()
	out, err := scanAccounts(rows)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return out, p, nil
}

const accountColumns = `id, condominium_id, account_type, title, category, amount_cents,
				due_date, status, settlement_date, supplier_payee, receipt_path,
				receipt_name, unit_id, payment_code, created_by, created_at, updated_at`

// accountWhere builds the WHERE clause (without the keyword) and its params
// shared by List and ListPage.
func accountWhere(condominiumID string, f services.AccountFilter) (string, []any) {
	params := []any{condominiumID}
	where := []string{"condominium_id = $1"}
	next := 2

	if f.Type != "" && f.Type.Valid() {
		where = append(where, fmt.Sprintf("account_type = $%d", next))
		params = append(params, f.Type)
		next++
	}
	if f.Category != "" {
		where = append(where, fmt.Sprintf("category = $%d", next))
		params = append(params, f.Category)
		next++
	}
	switch f.Status {
	case domain.StatusOverdue:
		// Derived status: pending with a past due date (FR-009).
		where = append(where, "status = 'pending' AND due_date < CURRENT_DATE")
	case domain.StatusPending, domain.StatusSettled, domain.StatusCanceled:
		where = append(where, fmt.Sprintf("status = $%d", next))
		params = append(params, f.Status)
		next++
	}
	if !f.Month.IsZero() {
		start := f.Month
		end := start.AddDate(0, 1, 0)
		where = append(where, fmt.Sprintf("due_date >= $%d AND due_date < $%d", next, next+1))
		params = append(params, start, end)
		next += 2
	}
	if f.Query != "" {
		pattern := "%" + escapeLike(f.Query) + "%"
		where = append(where, fmt.Sprintf("(title ILIKE $%d ESCAPE '\\' OR COALESCE(supplier_payee, '') ILIKE $%d ESCAPE '\\')", next, next))
		params = append(params, pattern)
		next++
	}
	return strings.Join(where, " AND "), params
}

// SummaryForMonth computes the monthly dashboard projection (research.md R5).
func (s *AccountStore) SummaryForMonth(ctx context.Context, condominiumID string, month time.Time) (services.MonthSummary, error) {
	start := month
	end := month.AddDate(0, 1, 0)
	var m services.MonthSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(amount_cents) FILTER (WHERE account_type = 'receivable' AND status <> 'canceled' AND due_date >= $2 AND due_date < $3), 0),
			COALESCE(SUM(amount_cents) FILTER (WHERE account_type = 'payable'    AND status <> 'canceled' AND due_date >= $2 AND due_date < $3), 0),
			COALESCE(SUM(amount_cents) FILTER (WHERE account_type = 'receivable' AND status = 'settled' AND settlement_date >= $2 AND settlement_date < $3), 0),
			COALESCE(SUM(amount_cents) FILTER (WHERE account_type = 'payable'    AND status = 'settled' AND settlement_date >= $2 AND settlement_date < $3), 0)
		FROM finance_accounts WHERE condominium_id = $1`, condominiumID, start, end).
		Scan(&m.TotalReceivableCents, &m.TotalPayableCents,
			&m.RealizedReceivableCents, &m.RealizedPayableCents)
	if err != nil {
		return services.MonthSummary{}, fmt.Errorf("summary for month: %w", err)
	}
	m.ProjectedBalanceCents = m.TotalReceivableCents - m.TotalPayableCents
	m.RealizedBalanceCents = m.RealizedReceivableCents - m.RealizedPayableCents
	return m, nil
}

func scanAccounts(rows *sql.Rows) ([]domain.FinancialAccount, error) {
	var out []domain.FinancialAccount
	for rows.Next() {
		var a domain.FinancialAccount
		var settlementDate sql.NullTime
		var supplier, receiptPath, receiptName, unitID, paymentCode sql.NullString
		if err := rows.Scan(&a.ID, &a.CondominiumID, &a.Type, &a.Title, &a.Category,
			&a.AmountCents, &a.DueDate, &a.Status, &settlementDate, &supplier,
			&receiptPath, &receiptName, &unitID, &paymentCode, &a.CreatedBy,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		if settlementDate.Valid {
			a.SettlementDate = &settlementDate.Time
		}
		a.SupplierPayee = supplier.String
		a.ReceiptPath = receiptPath.String
		a.ReceiptName = receiptName.String
		a.UnitID = unitID.String
		a.PaymentCode = paymentCode.String
		out = append(out, a)
	}
	return out, rows.Err()
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

// escapeLike escapes LIKE wildcards so user input is matched literally.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
