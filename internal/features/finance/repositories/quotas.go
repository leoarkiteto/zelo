package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
)

// QuotaStore persists budgets, issuances and quota charges. It implements
// services.QuotaStore and services.QuotaLinks.
type QuotaStore struct {
	db *sql.DB
}

// NewQuotaStore creates a QuotaStore.
func NewQuotaStore(db *sql.DB) *QuotaStore { return &QuotaStore{db: db} }

// mapQuotaErr maps PostgreSQL errors: a malformed UUID becomes ErrNotFound and
// the (condominium, year) unique violation becomes domain.ErrBudgetExists.
func mapQuotaErr(op string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return sharedstore.ErrNotFound
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "22P02":
			return sharedstore.ErrNotFound
		case "23505":
			if pe.ConstraintName == "quota_budgets_condo_year_unique" {
				return domain.ErrBudgetExists
			}
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

const budgetCols = `id, condominium_id, year, status, periodicity, due_day, reserve_rate_bp,
	approved_at, created_by, created_at, updated_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanBudget(r rowScanner) (services.Budget, error) {
	var b services.Budget
	var status, per string
	var approved sql.NullTime
	if err := r.Scan(&b.ID, &b.CondominiumID, &b.Year, &status, &per, &b.DueDay, &b.ReserveRateBP,
		&approved, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return b, err
	}
	b.Status, b.Periodicity = domain.BudgetStatus(status), domain.Periodicity(per)
	if approved.Valid {
		t := approved.Time
		b.ApprovedAt = &t
	}
	return b, nil
}

func loadLines(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, budgetID string) ([]services.BudgetLine, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, budget_id, description, category, amount_cents, position
		FROM quota_budget_lines WHERE budget_id = $1 ORDER BY position, id`, budgetID)
	if err != nil {
		return nil, mapQuotaErr("list budget lines", err)
	}
	defer rows.Close()
	var out []services.BudgetLine
	for rows.Next() {
		var l services.BudgetLine
		var cat string
		if err := rows.Scan(&l.ID, &l.BudgetID, &l.Description, &cat, &l.AmountCents, &l.Position); err != nil {
			return nil, err
		}
		l.Category = domain.Category(cat)
		out = append(out, l)
	}
	return out, rows.Err()
}

// CreateBudget inserts a draft budget; domain.ErrBudgetExists on a duplicate year.
func (s *QuotaStore) CreateBudget(ctx context.Context, b services.Budget) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO quota_budgets (condominium_id, year, periodicity, due_day, reserve_rate_bp, created_by)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		b.CondominiumID, b.Year, string(b.Periodicity), b.DueDay, b.ReserveRateBP, b.CreatedBy).Scan(&id)
	if err != nil {
		return "", mapQuotaErr("create budget", err)
	}
	return id, nil
}

// ListBudgets lists a condominium's budgets, newest year first.
func (s *QuotaStore) ListBudgets(ctx context.Context, condoID string) ([]services.BudgetSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.id, b.condominium_id, b.year, b.status, b.periodicity, b.due_day, b.reserve_rate_bp,
			b.approved_at, b.created_by, b.created_at, b.updated_at,
			(SELECT count(*) FROM quota_budget_lines l WHERE l.budget_id = b.id),
			(SELECT COALESCE(SUM(l.amount_cents), 0) FROM quota_budget_lines l WHERE l.budget_id = b.id)
		FROM quota_budgets b WHERE b.condominium_id = $1 ORDER BY b.year DESC`, condoID)
	if err != nil {
		return nil, mapQuotaErr("list budgets", err)
	}
	defer rows.Close()
	var out []services.BudgetSummary
	for rows.Next() {
		var b services.Budget
		var status, per string
		var approved sql.NullTime
		var sum services.BudgetSummary
		if err := rows.Scan(&b.ID, &b.CondominiumID, &b.Year, &status, &per, &b.DueDay, &b.ReserveRateBP,
			&approved, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt, &sum.LineCount, &sum.TotalCents); err != nil {
			return nil, err
		}
		b.Status, b.Periodicity = domain.BudgetStatus(status), domain.Periodicity(per)
		if approved.Valid {
			t := approved.Time
			b.ApprovedAt = &t
		}
		sum.Budget = b
		out = append(out, sum)
	}
	return out, rows.Err()
}

// GetBudget returns a budget with its lines.
func (s *QuotaStore) GetBudget(ctx context.Context, condoID, budgetID string) (services.BudgetDetail, error) {
	b, err := scanBudget(s.db.QueryRowContext(ctx,
		`SELECT `+budgetCols+` FROM quota_budgets WHERE condominium_id = $1 AND id = $2`, condoID, budgetID))
	if err != nil {
		return services.BudgetDetail{}, mapQuotaErr("get budget", err)
	}
	lines, err := loadLines(ctx, s.db, budgetID)
	if err != nil {
		return services.BudgetDetail{}, err
	}
	return services.BudgetDetail{Budget: b, Lines: lines}, nil
}

const issuanceSelect = `
	SELECT i.id, i.budget_id, b.year, b.periodicity, i.period_index, i.status,
		i.total_cents, i.base_cents, i.reserve_cents,
		(SELECT count(*) FROM quota_charges c WHERE c.issuance_id = i.id),
		(SELECT count(*) FROM quota_charges c JOIN finance_accounts fa ON fa.id = c.finance_account_id
			WHERE c.issuance_id = i.id AND fa.status = 'settled'),
		i.created_at, i.canceled_at
	FROM quota_issuances i JOIN quota_budgets b ON b.id = i.budget_id`

func scanIssuance(r rowScanner) (services.IssuanceSummary, error) {
	var i services.IssuanceSummary
	var per, status string
	var canceled sql.NullTime
	if err := r.Scan(&i.ID, &i.BudgetID, &i.Year, &per, &i.PeriodIndex, &status,
		&i.TotalCents, &i.BaseCents, &i.ReserveCents, &i.ChargeCount, &i.SettledCount,
		&i.CreatedAt, &canceled); err != nil {
		return i, err
	}
	i.Periodicity, i.Status = domain.Periodicity(per), domain.IssuanceStatus(status)
	if canceled.Valid {
		t := canceled.Time
		i.CanceledAt = &t
	}
	return i, nil
}

// ListIssuances lists the issuances of a budget by period.
func (s *QuotaStore) ListIssuances(ctx context.Context, condoID, budgetID string) ([]services.IssuanceSummary, error) {
	rows, err := s.db.QueryContext(ctx, issuanceSelect+`
		WHERE i.condominium_id = $1 AND i.budget_id = $2
		ORDER BY i.period_index, i.created_at`, condoID, budgetID)
	if err != nil {
		return nil, mapQuotaErr("list issuances", err)
	}
	defer rows.Close()
	var out []services.IssuanceSummary
	for rows.Next() {
		i, err := scanIssuance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// ActiveIssuanceFor returns the active issuance of a period, if any.
func (s *QuotaStore) ActiveIssuanceFor(ctx context.Context, condoID, budgetID string, periodIndex int) (services.IssuanceSummary, bool, error) {
	i, err := scanIssuance(s.db.QueryRowContext(ctx, issuanceSelect+`
		WHERE i.condominium_id = $1 AND i.budget_id = $2 AND i.period_index = $3 AND i.status = 'issued'`,
		condoID, budgetID, periodIndex))
	if errors.Is(err, sql.ErrNoRows) {
		return services.IssuanceSummary{}, false, nil
	}
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "22P02" {
			return services.IssuanceSummary{}, false, nil
		}
		return services.IssuanceSummary{}, false, fmt.Errorf("active issuance: %w", err)
	}
	return i, true, nil
}

func unitShares(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, condoID string) ([]domain.ShareInput, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, code, share_ppm FROM units WHERE condominium_id = $1 ORDER BY code, id`, condoID)
	if err != nil {
		return nil, mapQuotaErr("list unit shares", err)
	}
	defer rows.Close()
	var out []domain.ShareInput
	for rows.Next() {
		var u domain.ShareInput
		if err := rows.Scan(&u.UnitID, &u.Code, &u.SharePPM); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UnitReceivables returns every receivable of a unit with quota period info.
func (s *QuotaStore) UnitReceivables(ctx context.Context, condoID, unitID string) ([]services.StatementRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT fa.id, fa.condominium_id, fa.title, fa.category, fa.amount_cents, fa.due_date,
			fa.status, fa.settlement_date, fa.unit_id,
			qc.id IS NOT NULL, b.year, b.periodicity, i.period_index, u.code
		FROM finance_accounts fa
		LEFT JOIN units u ON u.id = fa.unit_id
		LEFT JOIN quota_charges qc ON qc.finance_account_id = fa.id
		LEFT JOIN quota_issuances i ON i.id = qc.issuance_id
		LEFT JOIN quota_budgets b ON b.id = i.budget_id
		WHERE fa.condominium_id = $1 AND fa.unit_id = $2 AND fa.account_type = 'receivable'
		ORDER BY fa.due_date, fa.title, fa.id`, condoID, unitID)
	if err != nil {
		return nil, mapQuotaErr("unit receivables", err)
	}
	defer rows.Close()
	var out []services.StatementRow
	for rows.Next() {
		var r services.StatementRow
		a := &r.Account
		var settlement sql.NullTime
		var year, idx sql.NullInt64
		var per, code sql.NullString
		if err := rows.Scan(&a.ID, &a.CondominiumID, &a.Title, &a.Category, &a.AmountCents, &a.DueDate,
			&a.Status, &settlement, &a.UnitID, &r.IsQuota, &year, &per, &idx, &code); err != nil {
			return nil, err
		}
		a.Type = domain.AccountTypeReceivable
		if settlement.Valid {
			t := settlement.Time
			a.SettlementDate = &t
		}
		if r.IsQuota {
			r.Period = domain.QuotaPeriod{Year: int(year.Int64), Periodicity: domain.Periodicity(per.String), Index: int(idx.Int64), UnitCode: code.String}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReceiptData returns a receivable joined with condominium, unit and quota data.
func (s *QuotaStore) ReceiptData(ctx context.Context, condoID, accountID string) (services.ReceiptRow, error) {
	var r services.ReceiptRow
	var unitID, unitCode, status, cur, per sql.NullString
	var settlement sql.NullTime
	var year, idx, base, reserve sql.NullInt64
	var isQuota bool
	err := s.db.QueryRowContext(ctx, `
		SELECT fa.id, c.name, c.currency, fa.unit_id, u.code, fa.title, fa.amount_cents,
			fa.status, fa.settlement_date,
			qc.id IS NOT NULL, b.year, b.periodicity, i.period_index, qc.base_cents, qc.reserve_cents
		FROM finance_accounts fa
		JOIN condominiums c ON c.id = fa.condominium_id
		LEFT JOIN units u ON u.id = fa.unit_id
		LEFT JOIN quota_charges qc ON qc.finance_account_id = fa.id
		LEFT JOIN quota_issuances i ON i.id = qc.issuance_id
		LEFT JOIN quota_budgets b ON b.id = i.budget_id
		WHERE fa.condominium_id = $1 AND fa.id = $2 AND fa.account_type = 'receivable'`,
		condoID, accountID).Scan(&r.AccountID, &r.CondominiumName, &cur, &unitID, &unitCode, &r.Title,
		&r.AmountCents, &status, &settlement, &isQuota, &year, &per, &idx, &base, &reserve)
	if err != nil {
		return services.ReceiptRow{}, mapQuotaErr("receipt data", err)
	}
	r.Currency = money.Currency(cur.String)
	r.UnitID, r.UnitCode = unitID.String, unitCode.String
	r.Status = domain.AccountStatus(status.String)
	if settlement.Valid {
		t := settlement.Time
		r.SettlementDate = &t
	}
	if isQuota {
		r.IsQuota = true
		r.Period = domain.QuotaPeriod{Year: int(year.Int64), Periodicity: domain.Periodicity(per.String), Index: int(idx.Int64), UnitCode: unitCode.String}
		r.BaseCents, r.ReserveCents = base.Int64, reserve.Int64
	}
	return r, nil
}

// CardData returns the dashboard rows (unit-scoped quota figures and today's approved budget).
func (s *QuotaStore) CardData(ctx context.Context, condoID string, unitIDs []string, today time.Time) (services.CardRows, error) {
	var out services.CardRows
	if len(unitIDs) > 0 {
		var amount sql.NullInt64
		var due sql.NullTime
		err := s.db.QueryRowContext(ctx, `
			SELECT fa.amount_cents, fa.due_date
			FROM finance_accounts fa JOIN quota_charges qc ON qc.finance_account_id = fa.id
			WHERE fa.condominium_id = $1 AND fa.unit_id = ANY($2::text[]::uuid[]) AND fa.status = 'pending'
				AND fa.due_date >= $3
			ORDER BY fa.due_date, fa.id LIMIT 1`, condoID, unitIDs, today).Scan(&amount, &due)
		switch {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return out, mapQuotaErr("card next pending", err)
		default:
			out.NextPending = &services.CardCharge{AmountCents: amount.Int64, DueDate: due.Time}
		}
		if err := s.db.QueryRowContext(ctx, `
			SELECT count(*), COALESCE(SUM(fa.amount_cents), 0)
			FROM finance_accounts fa JOIN quota_charges qc ON qc.finance_account_id = fa.id
			WHERE fa.condominium_id = $1 AND fa.unit_id = ANY($2::text[]::uuid[]) AND fa.status = 'pending'
				AND fa.due_date < $3`, condoID, unitIDs, today).
			Scan(&out.OverdueCount, &out.OverdueTotalCents); err != nil {
			return out, mapQuotaErr("card overdue", err)
		}
	}
	b, err := scanBudget(s.db.QueryRowContext(ctx, `SELECT `+budgetCols+`
		FROM quota_budgets WHERE condominium_id = $1 AND year = $2 AND status = 'approved'`, condoID, today.Year()))
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return out, mapQuotaErr("card budget", err)
	default:
		out.Budget = &b
	}
	return out, nil
}

// QuotaPeriods returns, for those of the account ids that are quota
// receivables (FR-16), the period and unit code to display, in one query.
func (s *QuotaStore) QuotaPeriods(ctx context.Context, ids []string) (map[string]domain.QuotaPeriod, error) {
	out := map[string]domain.QuotaPeriod{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT qc.finance_account_id, b.year, b.periodicity, i.period_index, COALESCE(u.code, '')
		FROM quota_charges qc
		JOIN quota_issuances i ON i.id = qc.issuance_id
		JOIN quota_budgets b ON b.id = i.budget_id
		LEFT JOIN units u ON u.id = qc.unit_id
		WHERE qc.finance_account_id = ANY($1::text[]::uuid[])`, ids)
	if err != nil {
		return nil, mapQuotaErr("quota periods", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, per string
		var p domain.QuotaPeriod
		if err := rows.Scan(&id, &p.Year, &per, &p.Index, &p.UnitCode); err != nil {
			return nil, err
		}
		p.Periodicity = domain.Periodicity(per)
		out[id] = p
	}
	return out, rows.Err()
}

// ---- lock-scoped transactions ----

// budgetTx is the part of the budget lock shared by the edit and issue views.
type budgetTx struct {
	ctx     context.Context //nolint:containedctx // the contract's callbacks carry no ctx; this is the transaction's
	tx      *sql.Tx
	condoID string
	budget  services.Budget
	lines   []services.BudgetLine
}

func (t *budgetTx) Budget() services.Budget      { return t.budget }
func (t *budgetTx) Lines() []services.BudgetLine { return t.lines }

func (t *budgetTx) reloadLines() error {
	l, err := loadLines(t.ctx, t.tx, t.budget.ID)
	if err != nil {
		return err
	}
	t.lines = l
	return nil
}

// lockBudget starts a transaction and locks the budget row FOR UPDATE.
func (s *QuotaStore) lockBudget(ctx context.Context, condoID, budgetID string) (*sql.Tx, *budgetTx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin locked budget: %w", err)
	}
	b, err := scanBudget(tx.QueryRowContext(ctx, `SELECT `+budgetCols+`
		FROM quota_budgets WHERE condominium_id = $1 AND id = $2 FOR UPDATE`, condoID, budgetID))
	if err != nil {
		_ = tx.Rollback()
		return nil, nil, mapQuotaErr("lock budget", err)
	}
	bt := &budgetTx{ctx: ctx, tx: tx, condoID: condoID, budget: b}
	// Separate statement: after waiting for the lock it sees what the previous holder committed.
	if err := bt.reloadLines(); err != nil {
		_ = tx.Rollback()
		return nil, nil, err
	}
	return tx, bt, nil
}

// WithLockedBudget runs fn in a transaction holding the budget row lock.
func (s *QuotaStore) WithLockedBudget(ctx context.Context, condoID, budgetID string,
	fn func(ctx context.Context, b services.LockedBudget) error) error {
	tx, bt, err := s.lockBudget(ctx, condoID, budgetID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(ctx, &lockedBudget{bt}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit locked budget: %w", err)
	}
	return nil
}

// WithLockedIssue runs fn in a transaction holding the budget row lock, with
// the issue-only write surface.
func (s *QuotaStore) WithLockedIssue(ctx context.Context, condoID, budgetID string,
	fn func(ctx context.Context, b services.LockedIssue) error) error {
	tx, bt, err := s.lockBudget(ctx, condoID, budgetID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(ctx, &lockedIssue{bt}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit locked issue: %w", err)
	}
	return nil
}

// lockedBudget is the budget-edit surface; it cannot insert issuances or charges.
type lockedBudget struct{ *budgetTx }

func (l *lockedBudget) touch() error {
	_, err := l.tx.ExecContext(l.ctx, `UPDATE quota_budgets SET updated_at = now() WHERE id = $1`, l.budget.ID)
	return err
}

func (l *lockedBudget) UpdateParams(p services.BudgetParams) error {
	if _, err := l.tx.ExecContext(l.ctx, `
		UPDATE quota_budgets SET periodicity = $2, due_day = $3, reserve_rate_bp = $4, updated_at = now()
		WHERE id = $1`, l.budget.ID, string(p.Periodicity), p.DueDay, p.ReserveRateBP); err != nil {
		return mapQuotaErr("update budget params", err)
	}
	l.budget.Periodicity, l.budget.DueDay, l.budget.ReserveRateBP = p.Periodicity, p.DueDay, p.ReserveRateBP
	return nil
}

func (l *lockedBudget) AddLine(line services.BudgetLine) (string, error) {
	var id string
	err := l.tx.QueryRowContext(l.ctx, `
		INSERT INTO quota_budget_lines (budget_id, description, category, amount_cents, position)
		VALUES ($1, $2, $3, $4, COALESCE((SELECT MAX(position) + 1 FROM quota_budget_lines WHERE budget_id = $1), 1))
		RETURNING id`, l.budget.ID, line.Description, string(line.Category), line.AmountCents).Scan(&id)
	if err != nil {
		return "", mapQuotaErr("add budget line", err)
	}
	if err := l.touch(); err != nil {
		return "", mapQuotaErr("touch budget", err)
	}
	return id, l.reloadLines()
}

func (l *lockedBudget) UpdateLine(line services.BudgetLine) error {
	res, err := l.tx.ExecContext(l.ctx, `
		UPDATE quota_budget_lines SET description = $3, category = $4, amount_cents = $5
		WHERE id = $1 AND budget_id = $2`, line.ID, l.budget.ID, line.Description, string(line.Category), line.AmountCents)
	if err != nil {
		return mapQuotaErr("update budget line", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sharedstore.ErrNotFound
	}
	if err := l.touch(); err != nil {
		return mapQuotaErr("touch budget", err)
	}
	return l.reloadLines()
}

func (l *lockedBudget) RemoveLine(lineID string) error {
	res, err := l.tx.ExecContext(l.ctx,
		`DELETE FROM quota_budget_lines WHERE id = $1 AND budget_id = $2`, lineID, l.budget.ID)
	if err != nil {
		return mapQuotaErr("remove budget line", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sharedstore.ErrNotFound
	}
	if err := l.touch(); err != nil {
		return mapQuotaErr("touch budget", err)
	}
	return l.reloadLines()
}

func (l *lockedBudget) SetStatus(status domain.BudgetStatus) error {
	if _, err := l.tx.ExecContext(l.ctx, `
		UPDATE quota_budgets SET status = $2,
			approved_at = CASE WHEN $2 = 'approved' THEN now() ELSE NULL END, updated_at = now()
		WHERE id = $1`, l.budget.ID, string(status)); err != nil {
		return mapQuotaErr("set budget status", err)
	}
	l.budget.Status = status
	return nil
}

func (l *lockedBudget) HasActiveIssuance() (bool, error) {
	var has bool
	err := l.tx.QueryRowContext(l.ctx,
		`SELECT EXISTS (SELECT 1 FROM quota_issuances WHERE budget_id = $1 AND status = 'issued')`,
		l.budget.ID).Scan(&has)
	if err != nil {
		return false, mapQuotaErr("active issuance check", err)
	}
	return has, nil
}

// lockedIssue is the issue surface: the only one that can insert issuances and charges.
type lockedIssue struct{ *budgetTx }

func (l *lockedIssue) Shares() ([]domain.ShareInput, error) {
	return unitShares(l.ctx, l.tx, l.condoID)
}

func (l *lockedIssue) InsertIssuance(in services.NewIssuance) (string, bool, error) {
	var id string
	err := l.tx.QueryRowContext(l.ctx, `
		INSERT INTO quota_issuances (condominium_id, budget_id, period_index, total_cents, base_cents, reserve_cents, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT DO NOTHING RETURNING id`,
		l.condoID, l.budget.ID, in.PeriodIndex, in.TotalCents, in.BaseCents, in.ReserveCents, in.CreatedBy).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, mapQuotaErr("insert issuance", err)
	}
	err = l.tx.QueryRowContext(l.ctx, `
		SELECT id FROM quota_issuances WHERE budget_id = $1 AND period_index = $2 AND status = 'issued'`,
		l.budget.ID, in.PeriodIndex).Scan(&id)
	if err != nil {
		return "", false, mapQuotaErr("existing issuance", err)
	}
	return id, false, nil
}

func (l *lockedIssue) InsertCharges(issuanceID string, charges []services.NewCharge) error {
	for _, c := range charges {
		accID, err := insertAccount(l.ctx, l.tx, domain.FinancialAccount{
			CondominiumID: l.condoID, Type: domain.AccountTypeReceivable, Title: c.Title,
			Category: domain.CategoryCondoFee, AmountCents: c.AmountCents, DueDate: c.DueDate,
			Status: domain.StatusPending, UnitID: c.UnitID, CreatedBy: c.CreatedBy,
		})
		if err != nil {
			return mapQuotaErr("insert quota account", err)
		}
		if _, err := l.tx.ExecContext(l.ctx, `
			INSERT INTO quota_charges (condominium_id, issuance_id, unit_id, finance_account_id, share_ppm, base_cents, reserve_cents)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			l.condoID, issuanceID, c.UnitID, accID, c.SharePPM, c.BaseCents, c.ReserveCents); err != nil {
			return mapQuotaErr("insert quota charge", err)
		}
	}
	return nil
}

// WithLockedIssuance runs fn in a transaction holding the issuance row lock
// (and nothing else: the budget is only read, never locked).
func (s *QuotaStore) WithLockedIssuance(ctx context.Context, condoID, issuanceID string,
	fn func(ctx context.Context, i services.LockedIssuance) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin locked issuance: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// Locks only the issuance row; the budget is never locked on this path.
	if _, err := tx.ExecContext(ctx, `
		SELECT 1 FROM quota_issuances i WHERE i.condominium_id = $1 AND i.id = $2 FOR UPDATE`,
		condoID, issuanceID); err != nil {
		return mapQuotaErr("lock issuance", err)
	}
	// Separate statement: after waiting for the lock it sees what the previous holder committed.
	iss, err := scanIssuance(tx.QueryRowContext(ctx, issuanceSelect+`
		WHERE i.condominium_id = $1 AND i.id = $2`, condoID, issuanceID))
	if err != nil {
		return mapQuotaErr("read issuance", err)
	}
	li := &lockedIssuance{ctx: ctx, tx: tx, condoID: condoID, iss: iss}
	if err := fn(ctx, li); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit locked issuance: %w", err)
	}
	return nil
}

type lockedIssuance struct {
	ctx     context.Context //nolint:containedctx // see budgetTx
	tx      *sql.Tx
	condoID string
	iss     services.IssuanceSummary
}

func (l *lockedIssuance) Issuance() services.IssuanceSummary { return l.iss }

func (l *lockedIssuance) CancelPendingCharges() (int, int, error) {
	var total int
	if err := l.tx.QueryRowContext(l.ctx,
		`SELECT count(*) FROM quota_charges WHERE issuance_id = $1`, l.iss.ID).Scan(&total); err != nil {
		return 0, 0, mapQuotaErr("count charges", err)
	}
	res, err := l.tx.ExecContext(l.ctx, `
		UPDATE finance_accounts SET status = 'canceled', updated_at = now()
		WHERE condominium_id = $2 AND status = 'pending'
			AND id IN (SELECT finance_account_id FROM quota_charges WHERE issuance_id = $1)`,
		l.iss.ID, l.condoID)
	if err != nil {
		return 0, 0, mapQuotaErr("cancel charges", err)
	}
	n, _ := res.RowsAffected()
	return int(n), total, nil
}

func (l *lockedIssuance) MarkCanceled(at time.Time) error {
	res, err := l.tx.ExecContext(l.ctx, `
		UPDATE quota_issuances SET status = 'canceled', canceled_at = $3
		WHERE id = $1 AND condominium_id = $2 AND status = 'issued'`, l.iss.ID, l.condoID, at)
	if err != nil {
		return mapQuotaErr("mark issuance canceled", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sharedstore.ErrNotFound
	}
	return nil
}
