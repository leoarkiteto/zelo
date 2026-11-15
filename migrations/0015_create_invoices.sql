-- 0015: supplier invoices (faturas) - suppliers, invoices, lines, VAT
-- breakdown, payments and attachments.
--
-- Money is integer cents, VAT rates are integer basis points and quantities
-- are integer milli-units (NFR-1). Every table carries condominium_id and the
-- audit columns. Child tables and the supplier / original-invoice references
-- use composite (condominium_id, ...) foreign keys, so a row can never point
-- at another condominium's data (FR-31).
--
-- The payment status is NOT stored: it is derived from state, payable_cents
-- and the sum of invoice_payments (see domain.Status and the list query).
--
-- Conditional fields of a prestacao de servicos and of a despesa mensal are
-- flat columns (service_*, monthly_*) rather than JSONB, so they stay
-- queryable and constrained.

CREATE TABLE suppliers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    nif TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT 'PT',
    address TEXT NOT NULL DEFAULT '',
    postal_code TEXT NOT NULL DEFAULT '',
    locality TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT suppliers_condo_id_unique UNIQUE (condominium_id, id),
    CONSTRAINT suppliers_name_check CHECK (btrim(name) <> '')
);

-- A NIF is unique per condominium (FR-6); suppliers without one are allowed.
CREATE UNIQUE INDEX idx_suppliers_condo_nif ON suppliers (condominium_id, nif) WHERE nif <> '';
CREATE INDEX idx_suppliers_condo_name ON suppliers (condominium_id, lower(name));

CREATE TABLE invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL REFERENCES condominiums(id) ON DELETE CASCADE,
    state TEXT NOT NULL DEFAULT 'draft',
    -- Hidden form key that makes a double submit create one invoice (FR-24).
    idempotency_key TEXT,

    document_type TEXT,
    expense_type TEXT,
    number TEXT,
    issue_date DATE,
    due_date DATE,
    operation_date DATE,
    supplier_id UUID,
    description TEXT,
    due_justification TEXT NOT NULL DEFAULT '',

    category TEXT,
    project TEXT NOT NULL DEFAULT '',
    cost_center TEXT NOT NULL DEFAULT '',
    reference_period TEXT,
    notes TEXT NOT NULL DEFAULT '',

    fiscal_region TEXT NOT NULL DEFAULT 'continental',

    -- Prestacao de servicos details (FR-10).
    service_description TEXT NOT NULL DEFAULT '',
    service_start_date DATE,
    service_end_date DATE,
    service_contract TEXT NOT NULL DEFAULT '',

    -- Despesa mensal details (FR-10).
    monthly_service_type TEXT NOT NULL DEFAULT '',
    monthly_billing_period TEXT NOT NULL DEFAULT '',
    monthly_start_date DATE,
    monthly_end_date DATE,
    monthly_contract_number TEXT NOT NULL DEFAULT '',
    monthly_meter_id TEXT NOT NULL DEFAULT '',
    monthly_cpe TEXT NOT NULL DEFAULT '',
    monthly_cui TEXT NOT NULL DEFAULT '',
    monthly_consumption_place TEXT NOT NULL DEFAULT '',
    monthly_recurring BOOLEAN NOT NULL DEFAULT false,

    -- Reserved for credit notes (FR-34); unused in phase 1.
    original_invoice_id UUID,

    -- Header totals, written only from domain.ComputeTotals.
    gross_cents BIGINT NOT NULL DEFAULT 0,
    discount_cents BIGINT NOT NULL DEFAULT 0,
    base_cents BIGINT NOT NULL DEFAULT 0,
    vat_cents BIGINT NOT NULL DEFAULT 0,
    total_cents BIGINT NOT NULL DEFAULT 0,
    withholding_cents BIGINT NOT NULL DEFAULT 0,
    payable_cents BIGINT NOT NULL DEFAULT 0,

    annulled_at TIMESTAMPTZ,
    annul_reason TEXT NOT NULL DEFAULT '',
    deleted_at TIMESTAMPTZ,

    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT invoices_condo_id_unique UNIQUE (condominium_id, id),
    CONSTRAINT invoices_idempotency_unique UNIQUE (condominium_id, idempotency_key),
    CONSTRAINT invoices_supplier_fk FOREIGN KEY (condominium_id, supplier_id)
        REFERENCES suppliers (condominium_id, id),
    CONSTRAINT invoices_original_fk FOREIGN KEY (condominium_id, original_invoice_id)
        REFERENCES invoices (condominium_id, id),

    CONSTRAINT invoices_state_check CHECK (state IN ('draft', 'registered', 'annulled')),
    CONSTRAINT invoices_document_type_check CHECK (document_type IS NULL OR document_type IN (
        'fatura', 'fatura_recibo', 'fatura_simplificada', 'nota_credito', 'nota_debito', 'outro')),
    CONSTRAINT invoices_expense_type_check CHECK (expense_type IS NULL OR expense_type IN (
        'compra_produtos', 'prestacao_servicos', 'despesa_mensal',
        'despesa_administrativa', 'manutencao', 'outra_despesa')),
    CONSTRAINT invoices_category_check CHECK (category IS NULL OR category IN (
        'compras_mercadorias', 'equipamentos_informatica', 'material_escritorio',
        'agua', 'eletricidade', 'gas', 'internet_telecomunicacoes', 'rendas',
        'seguros', 'contabilidade', 'consultoria', 'manutencao', 'limpeza',
        'transportes_deslocacoes', 'combustivel', 'despesas_bancarias',
        'outros_servicos', 'outras_despesas')),
    CONSTRAINT invoices_fiscal_region_check CHECK (fiscal_region IN ('continental', 'madeira', 'acores')),
    CONSTRAINT invoices_reference_period_check CHECK (
        reference_period IS NULL OR reference_period ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),

    -- A registered (or annulled) invoice is complete; only drafts may be partial.
    CONSTRAINT invoices_registered_complete_check CHECK (
        state = 'draft' OR (
            number IS NOT NULL AND issue_date IS NOT NULL AND supplier_id IS NOT NULL
            AND category IS NOT NULL AND description IS NOT NULL AND total_cents > 0)),
    CONSTRAINT invoices_withholding_check CHECK (withholding_cents BETWEEN 0 AND total_cents),
    CONSTRAINT invoices_payable_check CHECK (payable_cents = total_cents - withholding_cents),
    CONSTRAINT invoices_annulled_check CHECK ((state = 'annulled') = (annulled_at IS NOT NULL)),
    CONSTRAINT invoices_deleted_draft_check CHECK (deleted_at IS NULL OR state = 'draft')
);

CREATE INDEX idx_invoices_condo_issue_date ON invoices (condominium_id, issue_date);
CREATE INDEX idx_invoices_condo_supplier ON invoices (condominium_id, supplier_id);
CREATE INDEX idx_invoices_condo_number ON invoices (condominium_id, number);

-- Duplicate document key (FR-23): soft-deleted drafts free the key, annulled
-- invoices keep it.
CREATE UNIQUE INDEX idx_invoices_duplicate
    ON invoices (condominium_id, supplier_id, document_type, number)
    WHERE deleted_at IS NULL AND number IS NOT NULL;

CREATE TABLE invoice_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL,
    invoice_id UUID NOT NULL,
    position INT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    product TEXT NOT NULL DEFAULT '',
    reference TEXT NOT NULL DEFAULT '',
    unit TEXT NOT NULL DEFAULT '',
    quantity_milli BIGINT NOT NULL DEFAULT 0,
    unit_price_cents BIGINT NOT NULL DEFAULT 0,
    discount_cents BIGINT NOT NULL DEFAULT 0,
    base_cents BIGINT NOT NULL DEFAULT 0,
    treatment TEXT NOT NULL DEFAULT 'rate',
    rate_bp INT NOT NULL DEFAULT 0,
    exemption_code TEXT NOT NULL DEFAULT '',
    exemption_reason TEXT NOT NULL DEFAULT '',
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT invoice_items_invoice_fk FOREIGN KEY (condominium_id, invoice_id)
        REFERENCES invoices (condominium_id, id) ON DELETE CASCADE,
    CONSTRAINT invoice_items_treatment_check CHECK (treatment IN ('rate', 'exempt', 'reverse_charge')),
    CONSTRAINT invoice_items_rate_check CHECK (rate_bp >= 0)
);
CREATE INDEX idx_invoice_items_invoice ON invoice_items (condominium_id, invoice_id);

CREATE TABLE invoice_taxes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL,
    invoice_id UUID NOT NULL,
    position INT NOT NULL,
    treatment TEXT NOT NULL,
    rate_bp INT NOT NULL DEFAULT 0,
    base_cents BIGINT NOT NULL,
    vat_cents BIGINT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT invoice_taxes_invoice_fk FOREIGN KEY (condominium_id, invoice_id)
        REFERENCES invoices (condominium_id, id) ON DELETE CASCADE,
    CONSTRAINT invoice_taxes_treatment_check CHECK (treatment IN ('rate', 'exempt', 'reverse_charge')),
    CONSTRAINT invoice_taxes_rate_check CHECK (rate_bp >= 0)
);
CREATE INDEX idx_invoice_taxes_invoice ON invoice_taxes (condominium_id, invoice_id);

CREATE TABLE invoice_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL,
    invoice_id UUID NOT NULL,
    payment_date DATE NOT NULL,
    amount_cents BIGINT NOT NULL,
    method TEXT NOT NULL,
    reference TEXT NOT NULL DEFAULT '',
    bank_account TEXT NOT NULL DEFAULT '',
    justification TEXT NOT NULL DEFAULT '',
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT invoice_payments_invoice_fk FOREIGN KEY (condominium_id, invoice_id)
        REFERENCES invoices (condominium_id, id) ON DELETE CASCADE,
    CONSTRAINT invoice_payments_amount_check CHECK (amount_cents > 0),
    CONSTRAINT invoice_payments_method_check CHECK (method IN (
        'transferencia_bancaria', 'cartao', 'debito_direto', 'dinheiro',
        'multibanco', 'mb_way', 'outro'))
);
CREATE INDEX idx_invoice_payments_invoice ON invoice_payments (condominium_id, invoice_id);

CREATE TABLE invoice_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    condominium_id UUID NOT NULL,
    invoice_id UUID NOT NULL,
    stored_path TEXT NOT NULL,
    original_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT invoice_attachments_invoice_fk FOREIGN KEY (condominium_id, invoice_id)
        REFERENCES invoices (condominium_id, id) ON DELETE CASCADE,
    CONSTRAINT invoice_attachments_size_check CHECK (size_bytes >= 0)
);
CREATE INDEX idx_invoice_attachments_invoice ON invoice_attachments (condominium_id, invoice_id);
