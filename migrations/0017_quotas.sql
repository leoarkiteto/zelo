-- 0017: condominium quotas (quotas do condominio), phase 1.
--
-- Each unit carries a Permilagem (share) stored as integer parts per million
-- (0..1000000 = 0..1000 permille, three decimals). A budget (Orcamento) has
-- lines; an issuance (Emissao) of one period creates one receivable
-- finance_account per chargeable unit, linked through quota_charges.
-- All money is integer cents (NFR-1).
--
-- Cross-condominium safety follows 0015: UNIQUE (condominium_id, id) on the
-- parents and composite (condominium_id, ...) foreign keys on the children.

ALTER TABLE units ADD COLUMN share_ppm INTEGER NOT NULL DEFAULT 0
    CONSTRAINT units_share_ppm_check CHECK (share_ppm BETWEEN 0 AND 1000000);
ALTER TABLE units ADD CONSTRAINT units_condo_id_unique UNIQUE (condominium_id, id);

CREATE TABLE quota_budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    year INT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    periodicity TEXT NOT NULL,
    due_day INT NOT NULL,
    reserve_rate_bp INT NOT NULL DEFAULT 0,
    approved_at TIMESTAMPTZ,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT quota_budgets_condo_id_unique UNIQUE (condominium_id, id),
    CONSTRAINT quota_budgets_condo_year_unique UNIQUE (condominium_id, year),
    CONSTRAINT quota_budgets_status_check CHECK (status IN ('draft', 'approved')),
    CONSTRAINT quota_budgets_periodicity_check CHECK (periodicity IN ('monthly', 'quarterly')),
    CONSTRAINT quota_budgets_due_day_check CHECK (due_day BETWEEN 1 AND 28),
    CONSTRAINT quota_budgets_reserve_rate_check CHECK (reserve_rate_bp BETWEEN 0 AND 10000)
);

CREATE TABLE quota_budget_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES quota_budgets(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    category TEXT NOT NULL,
    amount_cents BIGINT NOT NULL,
    position INT NOT NULL DEFAULT 0,
    CONSTRAINT quota_budget_lines_category_check CHECK (category IN (
        'maintenance', 'cleaning', 'utilities', 'payroll', 'third_party_services', 'other')),
    CONSTRAINT quota_budget_lines_amount_check CHECK (amount_cents > 0)
);
CREATE INDEX idx_quota_budget_lines_budget ON quota_budget_lines (budget_id, position);

CREATE TABLE quota_issuances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    budget_id UUID NOT NULL,
    period_index INT NOT NULL,
    status TEXT NOT NULL DEFAULT 'issued',
    total_cents BIGINT NOT NULL,
    base_cents BIGINT NOT NULL,
    reserve_cents BIGINT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    canceled_at TIMESTAMPTZ,
    CONSTRAINT quota_issuances_condo_id_unique UNIQUE (condominium_id, id),
    CONSTRAINT quota_issuances_budget_fk FOREIGN KEY (condominium_id, budget_id)
        REFERENCES quota_budgets (condominium_id, id),
    CONSTRAINT quota_issuances_period_check CHECK (period_index >= 1),
    CONSTRAINT quota_issuances_status_check CHECK (status IN ('issued', 'canceled'))
);

-- At most one active issuance per budget period, enforced by the database
-- (NFR-2): concurrent issues of the same period cannot both succeed.
CREATE UNIQUE INDEX quota_issuances_one_active
    ON quota_issuances (budget_id, period_index) WHERE status = 'issued';

CREATE TABLE quota_charges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL,
    issuance_id UUID NOT NULL REFERENCES quota_issuances(id) ON DELETE CASCADE,
    unit_id UUID NOT NULL,
    finance_account_id UUID NOT NULL UNIQUE REFERENCES finance_accounts(id),
    -- Snapshot of the unit share at issue time (FR-17).
    share_ppm INT NOT NULL,
    base_cents BIGINT NOT NULL,
    reserve_cents BIGINT NOT NULL,
    CONSTRAINT quota_charges_unit_fk FOREIGN KEY (condominium_id, unit_id)
        REFERENCES units (condominium_id, id),
    CONSTRAINT quota_charges_issuance_unit_unique UNIQUE (issuance_id, unit_id)
);
CREATE INDEX idx_quota_charges_unit ON quota_charges (condominium_id, unit_id);

-- Extend the audit event allow-list (same pattern as 0013); keeps every
-- existing value.
ALTER TABLE audit_events DROP CONSTRAINT audit_events_event_type_check;
ALTER TABLE audit_events ADD CONSTRAINT audit_events_event_type_check CHECK (event_type IN (
    'sign_in', 'failed_sign_in', 'sign_out', 'account_locked',
    'role_granted', 'role_revoked', 'access_denied',
    'listing_created', 'listing_edited', 'listing_deleted',
    'category_created', 'category_renamed', 'category_deactivated',
    'account_created', 'account_edited', 'account_settled', 'account_canceled',
    'ticket_created', 'ticket_replied', 'ticket_closed',
    'condominium_currency_changed',
    'supplier_created', 'supplier_updated',
    'invoice_created', 'invoice_updated', 'invoice_registered',
    'invoice_draft_deleted', 'invoice_annulled', 'invoice_payment_added',
    'invoice_attachment_added', 'invoice_attachment_removed',
    'unit_created', 'unit_updated',
    'budget_created', 'budget_updated', 'budget_approved', 'budget_reopened',
    'quota_issued', 'quota_issuance_canceled'
));
