package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
)

// extraordinarySelect reads an extraordinary quota with its progress counters.
const extraordinarySelect = `
	SELECT e.id, e.condominium_id, e.description, e.total_cents, e.division, e.installments,
		e.first_due_date, e.status, e.created_by, e.created_at, e.canceled_at,
		(SELECT count(*) FROM quota_charges c JOIN quota_issuances i ON i.id = c.issuance_id
			WHERE i.extraordinary_id = e.id),
		(SELECT count(*) FROM quota_charges c JOIN quota_issuances i ON i.id = c.issuance_id
			JOIN finance_accounts fa ON fa.id = c.finance_account_id
			WHERE i.extraordinary_id = e.id AND fa.status = 'settled'),
		(SELECT count(*) FROM quota_issuances i
			WHERE i.extraordinary_id = e.id
				AND EXISTS (SELECT 1 FROM quota_charges c WHERE c.issuance_id = i.id)
				AND NOT EXISTS (SELECT 1 FROM quota_charges c JOIN finance_accounts fa ON fa.id = c.finance_account_id
					WHERE c.issuance_id = i.id AND fa.status <> 'settled'))
	FROM quota_extraordinaries e`

func scanExtraordinary(r rowScanner) (services.ExtraordinarySummary, error) {
	var e services.ExtraordinarySummary
	var division, status string
	var canceled sql.NullTime
	if err := r.Scan(&e.ID, &e.CondominiumID, &e.Description, &e.TotalCents, &division, &e.Installments,
		&e.FirstDueDate, &status, &e.CreatedBy, &e.CreatedAt, &canceled,
		&e.ChargeCount, &e.SettledCount, &e.SettledInstallments); err != nil {
		return e, err
	}
	e.Division, e.Status = services.Division(division), domain.IssuanceStatus(status)
	if canceled.Valid {
		t := canceled.Time
		e.CanceledAt = &t
	}
	return e, nil
}

// CreateExtraordinary inserts the quota, its installment issuances and all
// charges in one transaction (reserve is always 0).
func (s *QuotaStore) CreateExtraordinary(ctx context.Context, e services.NewExtraordinary, installments []services.NewInstallment) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin create extraordinary: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var id string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO quota_extraordinaries (condominium_id, description, total_cents, division, installments, first_due_date, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		e.CondominiumID, e.Description, e.TotalCents, string(e.Division), e.Installments, e.FirstDueDate, e.CreatedBy).Scan(&id); err != nil {
		return "", mapQuotaErr("insert extraordinary", err)
	}
	for _, in := range installments {
		var issID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO quota_issuances (condominium_id, extraordinary_id, period_index, total_cents, base_cents, reserve_cents, created_by)
			VALUES ($1, $2, $3, $4, $4, 0, $5) RETURNING id`,
			e.CondominiumID, id, in.Number, in.TotalCents, e.CreatedBy).Scan(&issID); err != nil {
			return "", mapQuotaErr("insert installment", err)
		}
		if err := insertCharges(ctx, tx, e.CondominiumID, issID, domain.CategoryExtraordinaryIncome, in.Charges); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit create extraordinary: %w", err)
	}
	return id, nil
}

// ListExtraordinaries lists a condominium's extraordinary quotas, newest first.
func (s *QuotaStore) ListExtraordinaries(ctx context.Context, condoID string) ([]services.ExtraordinarySummary, error) {
	rows, err := s.db.QueryContext(ctx, extraordinarySelect+`
		WHERE e.condominium_id = $1 ORDER BY e.created_at DESC, e.id`, condoID)
	if err != nil {
		return nil, mapQuotaErr("list extraordinaries", err)
	}
	defer rows.Close()
	var out []services.ExtraordinarySummary
	for rows.Next() {
		e, err := scanExtraordinary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetExtraordinary returns one extraordinary quota with its installments.
func (s *QuotaStore) GetExtraordinary(ctx context.Context, condoID, id string) (services.ExtraordinaryDetail, error) {
	e, err := scanExtraordinary(s.db.QueryRowContext(ctx, extraordinarySelect+`
		WHERE e.condominium_id = $1 AND e.id = $2`, condoID, id))
	if err != nil {
		return services.ExtraordinaryDetail{}, mapQuotaErr("get extraordinary", err)
	}
	rows, err := s.db.QueryContext(ctx, issuanceSelect+`
		WHERE i.condominium_id = $1 AND i.extraordinary_id = $2 ORDER BY i.period_index`, condoID, id)
	if err != nil {
		return services.ExtraordinaryDetail{}, mapQuotaErr("list installments", err)
	}
	defer rows.Close()
	d := services.ExtraordinaryDetail{ExtraordinarySummary: e}
	for rows.Next() {
		i, err := scanIssuance(rows)
		if err != nil {
			return services.ExtraordinaryDetail{}, err
		}
		d.Issuances = append(d.Issuances, i)
	}
	return d, rows.Err()
}

// WithLockedExtraordinary runs fn in a transaction holding the extraordinary
// quota row lock (and nothing else).
func (s *QuotaStore) WithLockedExtraordinary(ctx context.Context, condoID, id string,
	fn func(ctx context.Context, e services.LockedExtraordinary) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin locked extraordinary: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		SELECT 1 FROM quota_extraordinaries WHERE condominium_id = $1 AND id = $2 FOR UPDATE`, condoID, id); err != nil {
		return mapQuotaErr("lock extraordinary", err)
	}
	// Separate statement: after waiting for the lock it sees what the previous holder committed.
	ex, err := scanExtraordinary(tx.QueryRowContext(ctx, extraordinarySelect+`
		WHERE e.condominium_id = $1 AND e.id = $2`, condoID, id))
	if err != nil {
		return mapQuotaErr("read extraordinary", err)
	}
	le := &lockedExtraordinary{ctx: ctx, tx: tx, condoID: condoID, ex: ex}
	if err := fn(ctx, le); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit locked extraordinary: %w", err)
	}
	return nil
}

type lockedExtraordinary struct {
	ctx     context.Context //nolint:containedctx // see budgetTx
	tx      *sql.Tx
	condoID string
	ex      services.ExtraordinarySummary
}

func (l *lockedExtraordinary) Extraordinary() services.ExtraordinarySummary { return l.ex }

func (l *lockedExtraordinary) CancelPendingCharges() (int, int, error) {
	var total int
	if err := l.tx.QueryRowContext(l.ctx, `
		SELECT count(*) FROM quota_charges c JOIN quota_issuances i ON i.id = c.issuance_id
		WHERE i.extraordinary_id = $1`, l.ex.ID).Scan(&total); err != nil {
		return 0, 0, mapQuotaErr("count charges", err)
	}
	res, err := l.tx.ExecContext(l.ctx, `
		UPDATE finance_accounts SET status = 'canceled', updated_at = now()
		WHERE condominium_id = $2 AND status = 'pending'
			AND id IN (SELECT c.finance_account_id FROM quota_charges c
				JOIN quota_issuances i ON i.id = c.issuance_id WHERE i.extraordinary_id = $1)`,
		l.ex.ID, l.condoID)
	if err != nil {
		return 0, 0, mapQuotaErr("cancel charges", err)
	}
	n, _ := res.RowsAffected()
	return int(n), total, nil
}

func (l *lockedExtraordinary) MarkCanceled(at time.Time) error {
	res, err := l.tx.ExecContext(l.ctx, `
		UPDATE quota_extraordinaries SET status = 'canceled', canceled_at = $3
		WHERE id = $1 AND condominium_id = $2 AND status = 'issued'`, l.ex.ID, l.condoID, at)
	if err != nil {
		return mapQuotaErr("mark extraordinary canceled", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sharedstore.ErrNotFound
	}
	if _, err := l.tx.ExecContext(l.ctx, `
		UPDATE quota_issuances SET status = 'canceled', canceled_at = $3
		WHERE extraordinary_id = $1 AND condominium_id = $2 AND status = 'issued'`, l.ex.ID, l.condoID, at); err != nil {
		return mapQuotaErr("mark installments canceled", err)
	}
	return nil
}
