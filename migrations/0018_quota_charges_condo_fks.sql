-- Condominium-scoped composite foreign keys for quota_charges (follow-up to
-- 0017). A charge can no longer point at another condominium's issuance or
-- finance account. Same pattern as 0015 (UNIQUE (condominium_id, id) as the
-- composite FK target).

-- Redundant with the primary key, but required as a composite FK target.
ALTER TABLE finance_accounts
    ADD CONSTRAINT finance_accounts_condo_id_unique UNIQUE (condominium_id, id);

-- 0017 declared both single-column FKs inline (unnamed), so Postgres named
-- them <table>_<column>_fkey.
ALTER TABLE quota_charges DROP CONSTRAINT IF EXISTS quota_charges_issuance_id_fkey;
ALTER TABLE quota_charges DROP CONSTRAINT IF EXISTS quota_charges_finance_account_id_fkey;

-- ON DELETE CASCADE is kept for the issuance; the account FK keeps the
-- default NO ACTION, exactly as in 0017.
ALTER TABLE quota_charges
    ADD CONSTRAINT quota_charges_issuance_fk FOREIGN KEY (condominium_id, issuance_id)
        REFERENCES quota_issuances (condominium_id, id) ON DELETE CASCADE;
ALTER TABLE quota_charges
    ADD CONSTRAINT quota_charges_finance_account_fk FOREIGN KEY (condominium_id, finance_account_id)
        REFERENCES finance_accounts (condominium_id, id);
