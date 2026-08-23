-- 0009: finance - accounts payable & receivable.
-- Money is stored as integer cents; amounts must be positive.
-- 'overdue' is derived at read time (status='pending' AND due_date < CURRENT_DATE),
-- so only pending/settled/canceled are persisted.

CREATE TABLE finance_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    account_type TEXT NOT NULL,
    title TEXT NOT NULL,
    category TEXT NOT NULL,
    amount_cents BIGINT NOT NULL,
    due_date DATE NOT NULL,
    status TEXT NOT NULL,
    settlement_date DATE,
    supplier_payee TEXT,
    receipt_path TEXT,
    receipt_name TEXT,
    unit_id UUID REFERENCES units(id) ON DELETE SET NULL,
    payment_code TEXT,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Positive amounts only (FR-011).
ALTER TABLE finance_accounts ADD CONSTRAINT finance_accounts_amount_check
    CHECK (amount_cents > 0);

-- Valid account types and statuses; overdue is derived, never stored.
ALTER TABLE finance_accounts ADD CONSTRAINT finance_accounts_type_check
    CHECK (account_type IN ('payable', 'receivable'));
ALTER TABLE finance_accounts ADD CONSTRAINT finance_accounts_status_check
    CHECK (status IN ('pending', 'settled', 'canceled'));

-- Category must be valid for the account type.
ALTER TABLE finance_accounts ADD CONSTRAINT finance_accounts_category_check
    CHECK (
        (account_type = 'payable'   AND category IN ('maintenance', 'cleaning', 'utilities', 'payroll', 'third_party_services', 'other'))
        OR
        (account_type = 'receivable' AND category IN ('condo_fee', 'fine_interest', 'common_area_reservation', 'extraordinary_income'))
    );

-- Settling always requires a transaction date (FR-010).
ALTER TABLE finance_accounts ADD CONSTRAINT finance_accounts_settlement_check
    CHECK (
        (status = 'settled' AND settlement_date IS NOT NULL)
        OR (status <> 'settled' AND settlement_date IS NULL)
    );

-- Type-specific optional fields (data-model.md).
ALTER TABLE finance_accounts ADD CONSTRAINT finance_accounts_payable_fields_check
    CHECK (account_type <> 'payable' OR (unit_id IS NULL AND payment_code IS NULL));
ALTER TABLE finance_accounts ADD CONSTRAINT finance_accounts_receivable_fields_check
    CHECK (account_type <> 'receivable' OR (supplier_payee IS NULL AND receipt_path IS NULL AND receipt_name IS NULL));

-- List/filter/dashboard by condominium, type, status, and due date.
CREATE INDEX idx_finance_accounts_list ON finance_accounts (condominium_id, account_type, status, due_date);

-- Resident pendências: pending receivables per unit (FR-016).
CREATE INDEX idx_finance_accounts_unit_due ON finance_accounts (unit_id, due_date)
    WHERE account_type = 'receivable' AND status = 'pending';

-- Saldo realizado by settlement month.
CREATE INDEX idx_finance_accounts_settled ON finance_accounts (condominium_id, account_type, settlement_date);

-- Extend the audit event allow-list with finance events (same pattern as 0007).
ALTER TABLE audit_events DROP CONSTRAINT audit_events_event_type_check;
ALTER TABLE audit_events ADD CONSTRAINT audit_events_event_type_check CHECK (event_type IN (
    'sign_in', 'failed_sign_in', 'sign_out', 'account_locked',
    'role_granted', 'role_revoked', 'access_denied',
    'listing_created', 'listing_edited', 'listing_deleted',
    'category_created', 'category_renamed', 'category_deactivated',
    'account_created', 'account_edited', 'account_settled', 'account_canceled'
));
