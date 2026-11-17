package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/shared/pagination"
)

// statusCaseSQL mirrors domain.Status, rule for rule and in the same order
// (see the doc comment of domain.Status):
//
//	draft                    -> rascunho
//	annulled                 -> anulado
//	paid >= payable          -> pago
//	paid > 0                 -> parcialmente_pago
//	otherwise                -> pendente
//
// i is the invoices alias and pp.paid the sum of the payments. Keep both in
// sync; TestStatusSQLParity asserts they agree.
const statusCaseSQL = `CASE
		WHEN i.state = 'draft' THEN 'rascunho'
		WHEN i.state = 'annulled' THEN 'anulado'
		WHEN pp.paid >= i.payable_cents THEN 'pago'
		WHEN pp.paid > 0 THEN 'parcialmente_pago'
		ELSE 'pendente'
	END`

// listFrom is the shared FROM clause of the list queries.
const listFrom = `
	FROM invoices i
	LEFT JOIN suppliers s ON s.condominium_id = i.condominium_id AND s.id = i.supplier_id
	LEFT JOIN LATERAL (
		SELECT COALESCE(SUM(p.amount_cents), 0)::bigint AS paid
		FROM invoice_payments p
		WHERE p.condominium_id = i.condominium_id AND p.invoice_id = i.id
	) pp ON true`

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// buildWhere returns the WHERE clause and its arguments. Every filter is
// AND-combined; the condominium scope is always the first condition. An
// unparsable date filter is ignored.
func buildWhere(condoID string, f domain.ListFilter) (string, []any) {
	where := []string{"i.condominium_id = $1", "i.deleted_at IS NULL"}
	args := []any{condoID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if q := strings.TrimSpace(f.SupplierName); q != "" {
		add(`s.name ILIKE ? ESCAPE '\'`, likePattern(q))
	}
	if q := strings.ReplaceAll(strings.TrimSpace(f.NIF), " ", ""); q != "" {
		add(`s.nif ILIKE ? ESCAPE '\'`, likePattern(q))
	}
	if q := strings.TrimSpace(f.Number); q != "" {
		add(`i.number ILIKE ? ESCAPE '\'`, likePattern(q))
	}
	if f.DocumentType != "" {
		add(`i.document_type = ?`, f.DocumentType)
	}
	if f.ExpenseType != "" {
		add(`i.expense_type = ?`, f.ExpenseType)
	}
	if f.Category != "" {
		add(`i.category = ?`, f.Category)
	}
	if f.Status != "" {
		add(statusCaseSQL+` = ?`, f.Status)
	}
	if validDate(f.IssueFrom) {
		add(`i.issue_date >= ?::date`, f.IssueFrom)
	}
	if validDate(f.IssueTo) {
		add(`i.issue_date <= ?::date`, f.IssueTo)
	}
	if validDate(f.DueFrom) {
		add(`i.due_date >= ?::date`, f.DueFrom)
	}
	if validDate(f.DueTo) {
		add(`i.due_date <= ?::date`, f.DueTo)
	}
	if f.ReferencePeriod != "" {
		add(`i.reference_period = ?`, f.ReferencePeriod)
	}
	return strings.Join(where, " AND "), args
}

// List returns one page of the invoices of the condominium that match every
// filter (AND), ordered by issue date descending, with the paid sum and the
// derived status of each row. Totals cover the whole filtered set and exclude
// drafts and annulled invoices; RowCount counts every matching row. Drafts
// soft-deleted are never listed.
func (r *InvoiceStore) List(ctx context.Context, condoID string, f domain.ListFilter) (domain.ListResult, error) {
	f = f.Normalized()
	where, args := buildWhere(condoID, f)
	res := domain.ListResult{Page: f.Page, PageSize: f.PageSize}

	err := r.db.QueryRowContext(ctx, `
		SELECT count(*),
			count(*) FILTER (WHERE i.state = 'registered'),
			COALESCE(SUM(i.total_cents) FILTER (WHERE i.state = 'registered'), 0)::bigint,
			COALESCE(SUM(i.payable_cents) FILTER (WHERE i.state = 'registered'), 0)::bigint,
			COALESCE(SUM(pp.paid) FILTER (WHERE i.state = 'registered'), 0)::bigint`+
		listFrom+` WHERE `+where, args...).Scan(&res.RowCount, &res.Totals.Count,
		&res.Totals.TotalCents, &res.Totals.PayableCents, &res.Totals.PaidCents)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("list totals: %w", err)
	}
	res.Totals.DebtCents = domain.DebtCents(res.Totals.PayableCents, res.Totals.PaidCents)

	// Clamp a page past the last one to the last page (FR-4).
	pg := pagination.New(f.Page, res.RowCount)
	if f.PageSize == pagination.PageSize {
		res.Page = pg.Number
	} else {
		last := (res.RowCount + f.PageSize - 1) / f.PageSize
		if last < 1 {
			last = 1
		}
		res.Page = min(f.Page, last)
	}
	pageArgs := append(append([]any{}, args...), f.PageSize, (res.Page-1)*f.PageSize)
	limit := "$" + strconv.Itoa(len(args)+1)
	offset := "$" + strconv.Itoa(len(args)+2)
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.id, COALESCE(i.number, ''), COALESCE(s.name, ''), COALESCE(s.nif, ''),
			COALESCE(i.document_type, ''), COALESCE(i.expense_type, ''), COALESCE(i.category, ''),
			i.issue_date, i.due_date, COALESCE(i.reference_period, ''),
			i.total_cents, i.payable_cents, pp.paid, i.state, `+statusCaseSQL+`
		`+listFrom+` WHERE `+where+`
		ORDER BY i.issue_date DESC NULLS LAST, i.created_at DESC, i.id
		LIMIT `+limit+` OFFSET `+offset, pageArgs...)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row domain.ListRow
		var issue, due sql.NullTime
		var docType, expType, category, state, status string
		if err := rows.Scan(&row.ID, &row.Number, &row.SupplierName, &row.SupplierNIF,
			&docType, &expType, &category, &issue, &due, &row.ReferencePeriod,
			&row.TotalCents, &row.PayableCents, &row.PaidCents, &state, &status); err != nil {
			return domain.ListResult{}, fmt.Errorf("scan invoice row: %w", err)
		}
		row.DocumentType = domain.DocumentType(docType)
		row.ExpenseType = domain.ExpenseType(expType)
		row.Category = domain.Category(category)
		row.IssueDate, row.DueDate = dateOf(issue), dateOf(due)
		row.State = domain.State(state)
		row.Status = domain.PaymentStatus(status)
		row.DebtCents = domain.DebtCents(row.PayableCents, row.PaidCents)
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return domain.ListResult{}, fmt.Errorf("list invoices: %w", err)
	}
	return res, nil
}
