-- 0013: condominium currency (EUR or BRL, default BRL) and the audit event
-- types for the supplier-invoice module.

ALTER TABLE condominiums
    ADD COLUMN currency TEXT NOT NULL DEFAULT 'BRL'
    CONSTRAINT condominiums_currency_check CHECK (currency IN ('EUR', 'BRL'));

-- Extend the audit event allow-list (same pattern as 0010); keeps every
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
    'invoice_attachment_added', 'invoice_attachment_removed'
));
