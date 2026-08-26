-- 0010: tickets - resident requests, complaints, and suggestions.
-- Status is persisted only as open/closed (FR-004); closed tickets are
-- read-only (FR-008). Replies are append-only rows on open tickets.

CREATE TABLE tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    unit_id UUID REFERENCES units(id) ON DELETE SET NULL,
    category TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    closed_at TIMESTAMPTZ,
    closed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Categories are the three fixed keys from the spec (FR-001).
ALTER TABLE tickets ADD CONSTRAINT tickets_category_check
    CHECK (category IN ('repair', 'noise_complaint', 'assembly_topic'));

-- Statuses: only open and closed are persisted (FR-004).
ALTER TABLE tickets ADD CONSTRAINT tickets_status_check
    CHECK (status IN ('open', 'closed'));

-- Description is required, 1-2000 characters (FR-002).
ALTER TABLE tickets ADD CONSTRAINT tickets_description_length_check
    CHECK (char_length(description) BETWEEN 1 AND 2000);

-- Closing records the actor and date together; open tickets have neither (FR-007).
ALTER TABLE tickets ADD CONSTRAINT tickets_closed_fields_check
    CHECK (
        (status = 'closed' AND closed_at IS NOT NULL AND closed_by IS NOT NULL)
        OR (status = 'open' AND closed_at IS NULL AND closed_by IS NULL)
    );

CREATE TABLE ticket_replies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Reply content is required, 1-2000 characters.
ALTER TABLE ticket_replies ADD CONSTRAINT ticket_replies_content_length_check
    CHECK (char_length(content) BETWEEN 1 AND 2000);

-- Syndic inbox: open tickets handled oldest first.
CREATE INDEX idx_tickets_inbox ON tickets (condominium_id, status, created_at);

-- Resident list: own tickets newest first.
CREATE INDEX idx_tickets_author ON tickets (author_id, created_at);

-- Detail page: replies in chronological order.
CREATE INDEX idx_ticket_replies_ticket ON ticket_replies (ticket_id, created_at);

-- Extend the audit event allow-list with ticket events (same pattern as 0007/0009).
ALTER TABLE audit_events DROP CONSTRAINT audit_events_event_type_check;
ALTER TABLE audit_events ADD CONSTRAINT audit_events_event_type_check CHECK (event_type IN (
    'sign_in', 'failed_sign_in', 'sign_out', 'account_locked',
    'role_granted', 'role_revoked', 'access_denied',
    'listing_created', 'listing_edited', 'listing_deleted',
    'category_created', 'category_renamed', 'category_deactivated',
    'account_created', 'account_edited', 'account_settled', 'account_canceled',
    'ticket_created', 'ticket_replied', 'ticket_closed'
));
