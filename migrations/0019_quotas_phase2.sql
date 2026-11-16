-- 0019: condominium quotas, phase 2: late fees (multa e juros de mora),
-- extraordinary quotas and their audit events.
--
-- Rates are stored as integer basis points (1 bp = 0.01 %), money as integer
-- cents (NFR-1). Cross-condominium safety follows 0015/0018: composite
-- (condominium_id, ...) foreign keys onto UNIQUE (condominium_id, id) targets.

-- Per-condominium late-fee settings. Defaults are the Brazilian ones (multa 2 %,
-- juros 1 % per month, no grace); condominiums already on EUR start at 0/0.
-- The "BRL multa <= 2 %" cap is a service rule (model.MaxBRLLateFeeBP), not a
-- CHECK, because the currency can change.
ALTER TABLE condominiums
    ADD COLUMN late_fee_bp INTEGER NOT NULL DEFAULT 200
        CONSTRAINT condominiums_late_fee_bp_check CHECK (late_fee_bp BETWEEN 0 AND 10000),
    ADD COLUMN late_interest_bp INTEGER NOT NULL DEFAULT 100
        CONSTRAINT condominiums_late_interest_bp_check CHECK (late_interest_bp BETWEEN 0 AND 10000),
    ADD COLUMN late_grace_days INTEGER NOT NULL DEFAULT 0
        CONSTRAINT condominiums_late_grace_days_check CHECK (late_grace_days BETWEEN 0 AND 60);

UPDATE condominiums SET late_fee_bp = 0, late_interest_bp = 0 WHERE currency = 'EUR';

-- Extraordinary quota (quota extraordinaria): one parent row, N installment
-- issuances (quota_issuances.extraordinary_id).
CREATE TABLE quota_extraordinaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    total_cents BIGINT NOT NULL,
    division TEXT NOT NULL,
    installments INT NOT NULL,
    first_due_date DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'issued',
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    canceled_at TIMESTAMPTZ,
    CONSTRAINT quota_extraordinaries_condo_id_unique UNIQUE (condominium_id, id),
    CONSTRAINT quota_extraordinaries_total_check CHECK (total_cents > 0),
    CONSTRAINT quota_extraordinaries_division_check CHECK (division IN ('share', 'equal')),
    CONSTRAINT quota_extraordinaries_installments_check CHECK (installments BETWEEN 1 AND 24),
    CONSTRAINT quota_extraordinaries_first_due_day_check CHECK (EXTRACT(DAY FROM first_due_date) BETWEEN 1 AND 28),
    CONSTRAINT quota_extraordinaries_status_check CHECK (status IN ('issued', 'canceled'))
);

-- An issuance now belongs to a budget (ordinary) or to an extraordinary quota
-- (installment); the kind is derived, there is no kind column. The existing
-- quota_issuances_one_active index is unchanged (NULL budget_id never collides).
ALTER TABLE quota_issuances ALTER COLUMN budget_id DROP NOT NULL;
ALTER TABLE quota_issuances ADD COLUMN extraordinary_id UUID;
ALTER TABLE quota_issuances
    ADD CONSTRAINT quota_issuances_extraordinary_fk FOREIGN KEY (condominium_id, extraordinary_id)
        REFERENCES quota_extraordinaries (condominium_id, id);
ALTER TABLE quota_issuances
    ADD CONSTRAINT quota_issuances_owner_check CHECK (num_nonnulls(budget_id, extraordinary_id) = 1);
-- period_index is the installment number for extraordinary issuances.
ALTER TABLE quota_issuances
    ADD CONSTRAINT quota_issuances_extraordinary_installment_unique UNIQUE (extraordinary_id, period_index);

-- Late fee (mora) recorded when a late quota is settled with "apply": a
-- separate receivable (late_fee_account_id) next to the settled quota account.
-- The UNIQUE columns guarantee at most one mora per quota (NFR-2).
CREATE TABLE quota_late_fees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    quota_account_id UUID NOT NULL,
    late_fee_account_id UUID NOT NULL,
    multa_cents BIGINT NOT NULL,
    juros_cents BIGINT NOT NULL,
    days_late INT NOT NULL,
    multa_bp INT NOT NULL,
    juros_bp INT NOT NULL,
    grace_days INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT quota_late_fees_quota_account_unique UNIQUE (quota_account_id),
    CONSTRAINT quota_late_fees_late_fee_account_unique UNIQUE (late_fee_account_id),
    CONSTRAINT quota_late_fees_quota_account_fk FOREIGN KEY (condominium_id, quota_account_id)
        REFERENCES finance_accounts (condominium_id, id),
    CONSTRAINT quota_late_fees_late_fee_account_fk FOREIGN KEY (condominium_id, late_fee_account_id)
        REFERENCES finance_accounts (condominium_id, id),
    CONSTRAINT quota_late_fees_amounts_check CHECK (multa_cents >= 0 AND juros_cents >= 0 AND days_late >= 0)
);

-- Extend the audit event allow-list (same pattern as 0013/0017); keeps every
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
    'quota_issued', 'quota_issuance_canceled',
    'late_fee_settings_changed', 'late_fee_applied', 'late_fee_waived',
    'extraordinary_quota_created', 'extraordinary_quota_canceled'
));
