-- 0016: a draft may hold lines or a withholding that do not compute (the header
-- totals stay 0 and the withholding is stored as entered). The two total
-- consistency checks of 0015 now apply only to invoices that are not drafts
-- (a draft passes; any other state must satisfy the original condition).
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_withholding_check;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_payable_check;

ALTER TABLE invoices ADD CONSTRAINT invoices_withholding_check
    CHECK (state = 'draft' OR (withholding_cents BETWEEN 0 AND total_cents));
ALTER TABLE invoices ADD CONSTRAINT invoices_payable_check
    CHECK (state = 'draft' OR (payable_cents = total_cents - withholding_cents));
