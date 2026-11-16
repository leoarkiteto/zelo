package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

const quotaAccountSelect = `
	SELECT fa.id, fa.condominium_id, COALESCE(fa.unit_id::text, ''), fa.title, fa.amount_cents,
		fa.due_date, fa.status,
		EXISTS (SELECT 1 FROM quota_charges qc WHERE qc.finance_account_id = fa.id)
	FROM finance_accounts fa
	WHERE fa.condominium_id = $1 AND fa.id = $2 AND fa.account_type = 'receivable'`

func scanQuotaAccount(r rowScanner) (services.QuotaAccount, error) {
	var a services.QuotaAccount
	var status string
	if err := r.Scan(&a.ID, &a.CondominiumID, &a.UnitID, &a.Title, &a.AmountCents, &a.DueDate, &status, &a.IsQuota); err != nil {
		return a, err
	}
	a.Status = domain.AccountStatus(status)
	return a, nil
}

// QuotaAccount reads a receivable and whether it is a quota charge, without locking.
func (s *QuotaStore) QuotaAccount(ctx context.Context, condoID, accountID string) (services.QuotaAccount, error) {
	a, err := scanQuotaAccount(s.db.QueryRowContext(ctx, quotaAccountSelect, condoID, accountID))
	if err != nil {
		return services.QuotaAccount{}, mapQuotaErr("quota account", err)
	}
	return a, nil
}

// WithLockedQuotaAccount runs fn in a transaction holding the receivable row
// lock (and nothing else: no issuance, budget or extraordinary row is locked,
// so the order of the cancel paths cannot be inverted).
func (s *QuotaStore) WithLockedQuotaAccount(ctx context.Context, condoID, accountID string,
	fn func(ctx context.Context, a services.LockedQuotaAccount) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin locked quota account: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		SELECT 1 FROM finance_accounts
		WHERE condominium_id = $1 AND id = $2 AND account_type = 'receivable' FOR UPDATE`, condoID, accountID); err != nil {
		return mapQuotaErr("lock quota account", err)
	}
	// Separate statement: after waiting for the lock it sees what the previous holder committed.
	a, err := scanQuotaAccount(tx.QueryRowContext(ctx, quotaAccountSelect, condoID, accountID))
	if err != nil {
		return mapQuotaErr("read quota account", err)
	}
	if err := fn(ctx, &lockedQuotaAccount{ctx: ctx, tx: tx, acc: a}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit locked quota account: %w", err)
	}
	return nil
}

type lockedQuotaAccount struct {
	ctx context.Context //nolint:containedctx // see budgetTx
	tx  *sql.Tx
	acc services.QuotaAccount
}

func (l *lockedQuotaAccount) Account() services.QuotaAccount { return l.acc }

func (l *lockedQuotaAccount) Settle(on time.Time) error {
	res, err := l.tx.ExecContext(l.ctx, `
		UPDATE finance_accounts SET status = 'settled', settlement_date = $3, updated_at = now()
		WHERE id = $1 AND condominium_id = $2 AND status = 'pending'`, l.acc.ID, l.acc.CondominiumID, on)
	if err != nil {
		return mapQuotaErr("settle quota account", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotEditable
	}
	l.acc.Status = domain.StatusSettled
	return nil
}

func (l *lockedQuotaAccount) InsertLateFee(in services.NewLateFee) (string, error) {
	d := in.Detail
	id, err := insertAccount(l.ctx, l.tx, domain.FinancialAccount{
		CondominiumID: l.acc.CondominiumID, Type: domain.AccountTypeReceivable, Title: in.Title,
		Category: domain.CategoryFineInterest, AmountCents: d.FineCents + d.InterestCents, DueDate: in.DueDate,
		Status: domain.StatusPending, UnitID: l.acc.UnitID, CreatedBy: in.CreatedBy,
	})
	if err != nil {
		return "", mapQuotaErr("insert late fee account", err)
	}
	if _, err := l.tx.ExecContext(l.ctx, `
		INSERT INTO quota_late_fees (condominium_id, quota_account_id, late_fee_account_id,
			multa_cents, juros_cents, days_late, multa_bp, juros_bp, grace_days)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		l.acc.CondominiumID, l.acc.ID, id, d.FineCents, d.InterestCents, d.DaysLate, d.FeeBP, d.InterestBP, d.GraceDays); err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" && pe.ConstraintName == "quota_late_fees_quota_account_unique" {
			return "", domain.ErrNotEditable // a late fee already exists for this quota
		}
		return "", mapQuotaErr("insert late fee", err)
	}
	return id, nil
}

// LateFeeOrigins returns, for those of the account ids that are late fee
// receivables, the origin quota period and the stored breakdown, in one query.
func (s *QuotaStore) LateFeeOrigins(ctx context.Context, ids []string) (map[string]domain.LateFeeOrigin, error) {
	out := map[string]domain.LateFeeOrigin{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT lf.late_fee_account_id, lf.multa_cents, lf.juros_cents, lf.days_late, lf.multa_bp, lf.juros_bp, lf.grace_days,
			`+periodCols+`, COALESCE(u.code, '')
		FROM quota_late_fees lf
		JOIN quota_charges qc ON qc.finance_account_id = lf.quota_account_id
		JOIN quota_issuances i ON i.id = qc.issuance_id`+periodJoins+`
		LEFT JOIN units u ON u.id = qc.unit_id
		WHERE lf.late_fee_account_id = ANY($1::text[]::uuid[])`, ids)
	if err != nil {
		return nil, mapQuotaErr("late fee origins", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, code string
		var d domain.LateFeeDetail
		var ps periodScan
		dest := append([]any{&id, &d.FineCents, &d.InterestCents, &d.DaysLate, &d.FeeBP, &d.InterestBP, &d.GraceDays}, ps.dest()...)
		if err := rows.Scan(append(dest, &code)...); err != nil {
			return nil, err
		}
		out[id] = domain.LateFeeOrigin{Origin: ps.period(code), Detail: d}
	}
	return out, rows.Err()
}

// DebtorRows returns one row per unit with a pending receivable due before
// today, biggest total first. The overdue receivables and the active occupants
// are each pre-aggregated per unit in a CTE and joined once, so a unit with
// several occupants and several overdue receivables is not fanned out and the
// number of queries does not depend on the number of units (NFR-5, R7).
func (s *QuotaStore) DebtorRows(ctx context.Context, condoID string, today time.Time) ([]services.DebtorRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH overdue AS (
			SELECT unit_id, count(*) AS n, SUM(amount_cents) AS total, MIN(due_date) AS oldest
			FROM finance_accounts
			WHERE condominium_id = $1 AND account_type = 'receivable' AND status = 'pending'
				AND due_date < $2 AND unit_id IS NOT NULL
			GROUP BY unit_id
		), occ AS (
			SELECT o.unit_id,
				string_agg(u.email || E'\t' || o.occupancy_type::text, E'\n' ORDER BY o.occupancy_type::text, u.email) AS list
			FROM unit_occupancies o
			JOIN users u ON u.id = o.user_id
			WHERE o.ended_at IS NULL AND o.unit_id IN (SELECT unit_id FROM overdue)
			GROUP BY o.unit_id
		)
		SELECT un.id, un.code, d.n, d.total, d.oldest, COALESCE(occ.list, '')
		FROM overdue d
		JOIN units un ON un.id = d.unit_id AND un.condominium_id = $1
		LEFT JOIN occ ON occ.unit_id = d.unit_id
		ORDER BY d.total DESC, un.code, un.id`, condoID, today)
	if err != nil {
		return nil, mapQuotaErr("debtor rows", err)
	}
	defer rows.Close()
	var out []services.DebtorRow
	for rows.Next() {
		var r services.DebtorRow
		var list string
		if err := rows.Scan(&r.UnitID, &r.UnitCode, &r.OverdueCount, &r.OverdueTotalCents, &r.OldestDue, &list); err != nil {
			return nil, err
		}
		for _, line := range strings.Split(list, "\n") {
			if email, typ, ok := strings.Cut(line, "\t"); ok {
				r.Occupants = append(r.Occupants, services.DebtorOccupant{Email: email, Type: model.OccupancyType(typ)})
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// OverdueQuotaRows lists the pending quota charges due before today.
func (s *QuotaStore) OverdueQuotaRows(ctx context.Context, condoID string, today time.Time) ([]services.OverdueQuota, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT fa.unit_id, fa.amount_cents, fa.due_date
		FROM finance_accounts fa
		JOIN quota_charges qc ON qc.finance_account_id = fa.id
		WHERE fa.condominium_id = $1 AND fa.status = 'pending' AND fa.due_date < $2 AND fa.unit_id IS NOT NULL`,
		condoID, today)
	if err != nil {
		return nil, mapQuotaErr("overdue quota rows", err)
	}
	defer rows.Close()
	var out []services.OverdueQuota
	for rows.Next() {
		var q services.OverdueQuota
		if err := rows.Scan(&q.UnitID, &q.AmountCents, &q.DueDate); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// ReserveTotals sums the reserve part of the quota charges by account status
// (canceled charges count nowhere) and the part settled in the calendar year.
func (s *QuotaStore) ReserveTotals(ctx context.Context, condoID string, year int) (services.ReserveTotals, error) {
	var t services.ReserveTotals
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(qc.reserve_cents) FILTER (WHERE fa.status = 'settled'), 0),
			COALESCE(SUM(qc.reserve_cents) FILTER (WHERE fa.status = 'pending'), 0),
			COALESCE(SUM(qc.reserve_cents) FILTER (WHERE fa.status = 'settled'
				AND fa.settlement_date >= make_date($2, 1, 1) AND fa.settlement_date < make_date($2 + 1, 1, 1)), 0)
		FROM quota_charges qc
		JOIN finance_accounts fa ON fa.id = qc.finance_account_id
		WHERE qc.condominium_id = $1`, condoID, year).Scan(&t.SettledCents, &t.PendingCents, &t.SettledYearCents)
	if err != nil {
		return services.ReserveTotals{}, mapQuotaErr("reserve totals", err)
	}
	return t, nil
}
