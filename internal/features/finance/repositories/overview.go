package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
)

var _ services.OverviewStore = (*AccountStore)(nil)

// MonthlyRealized returns settled flows grouped by settlement month (sparse).
// Month keys are UTC first-of-month values.
func (s *AccountStore) MonthlyRealized(ctx context.Context, condominiumID string, start, end time.Time) ([]services.MonthFlow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT EXTRACT(YEAR FROM settlement_date)::int,
		       EXTRACT(MONTH FROM settlement_date)::int,
		       COALESCE(SUM(amount_cents) FILTER (WHERE account_type = 'receivable'), 0),
		       COALESCE(SUM(amount_cents) FILTER (WHERE account_type = 'payable'), 0)
		FROM finance_accounts
		WHERE condominium_id = $1 AND status = 'settled'
		  AND settlement_date >= $2 AND settlement_date < $3
		GROUP BY 1, 2
		ORDER BY 1, 2`, condominiumID, start, end)
	if err != nil {
		return nil, fmt.Errorf("monthly realized: %w", err)
	}
	defer rows.Close()
	var out []services.MonthFlow
	for rows.Next() {
		var y, m int
		var f services.MonthFlow
		if err := rows.Scan(&y, &m, &f.InCents, &f.OutCents); err != nil {
			return nil, fmt.Errorf("scan monthly realized: %w", err)
		}
		f.Month = time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("monthly realized rows: %w", err)
	}
	return out, nil
}

// RealizedByCategory returns settled totals grouped by type and category.
func (s *AccountStore) RealizedByCategory(ctx context.Context, condominiumID string, start, end time.Time) ([]services.CategoryTotal, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT account_type, category, SUM(amount_cents)
		FROM finance_accounts
		WHERE condominium_id = $1 AND status = 'settled'
		  AND settlement_date >= $2 AND settlement_date < $3
		GROUP BY account_type, category
		ORDER BY account_type, category`, condominiumID, start, end)
	if err != nil {
		return nil, fmt.Errorf("realized by category: %w", err)
	}
	defer rows.Close()
	var out []services.CategoryTotal
	for rows.Next() {
		var t, c string
		var cents int64
		if err := rows.Scan(&t, &c, &cents); err != nil {
			return nil, fmt.Errorf("scan realized by category: %w", err)
		}
		out = append(out, services.CategoryTotal{
			Type: domain.AccountType(t), Category: domain.Category(c), Cents: cents,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("realized by category rows: %w", err)
	}
	return out, nil
}

// CurrentPosition returns pending receivable/payable due in
// [monthStart, monthEnd) and pending receivables due before today.
func (s *AccountStore) CurrentPosition(ctx context.Context, condominiumID string, monthStart, monthEnd, today time.Time) (services.CurrentPosition, error) {
	var p services.CurrentPosition
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(amount_cents) FILTER (WHERE account_type = 'receivable' AND due_date >= $2 AND due_date < $3), 0),
			COALESCE(SUM(amount_cents) FILTER (WHERE account_type = 'payable'    AND due_date >= $2 AND due_date < $3), 0),
			COALESCE(SUM(amount_cents) FILTER (WHERE account_type = 'receivable' AND due_date < $4), 0)
		FROM finance_accounts
		WHERE condominium_id = $1 AND status = 'pending'`,
		condominiumID, monthStart, monthEnd, today).
		Scan(&p.ReceivableCents, &p.PayableCents, &p.OverdueCents)
	if err != nil {
		return services.CurrentPosition{}, fmt.Errorf("current position: %w", err)
	}
	return p, nil
}

// SettlementYears returns the distinct settlement years, descending.
func (s *AccountStore) SettlementYears(ctx context.Context, condominiumID string) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT EXTRACT(YEAR FROM settlement_date)::int AS y
		FROM finance_accounts
		WHERE condominium_id = $1 AND status = 'settled' AND settlement_date IS NOT NULL
		ORDER BY y DESC`, condominiumID)
	if err != nil {
		return nil, fmt.Errorf("settlement years: %w", err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			return nil, fmt.Errorf("scan settlement year: %w", err)
		}
		out = append(out, y)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("settlement years rows: %w", err)
	}
	return out, nil
}
