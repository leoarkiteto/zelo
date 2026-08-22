CREATE TABLE service_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Case-insensitive unique name among active categories per condominium; a
-- deactivated name can be reused later.
CREATE UNIQUE INDEX service_categories_active_name_uniq
    ON service_categories (condominium_id, lower(name))
    WHERE active = TRUE;

CREATE INDEX idx_service_categories_condominium ON service_categories (condominium_id);

CREATE TABLE service_provider_listings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES service_categories(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    phone TEXT NOT NULL,
    phone_digits TEXT NOT NULL,
    notes TEXT,
    recommended_by_unit_id UUID REFERENCES units(id) ON DELETE SET NULL,
    recommended_by_unit_code TEXT NOT NULL,
    submitted_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Duplicate-phone detection per condominium (FR-021).
CREATE INDEX idx_listings_condominium_phone ON service_provider_listings (condominium_id, phone_digits);

-- Category filtering (FR-009).
CREATE INDEX idx_listings_condominium_category ON service_provider_listings (condominium_id, category_id);

-- Extend the audit event allow-list with directory events (migration 0006
-- created the original CHECK constraint).
ALTER TABLE audit_events DROP CONSTRAINT audit_events_event_type_check;
ALTER TABLE audit_events ADD CONSTRAINT audit_events_event_type_check CHECK (event_type IN (
    'sign_in', 'failed_sign_in', 'sign_out', 'account_locked',
    'role_granted', 'role_revoked', 'access_denied',
    'listing_created', 'listing_edited', 'listing_deleted',
    'category_created', 'category_renamed', 'category_deactivated'
));
