-- 0011: tickets - add a title and the 'other' category.
-- Forward-only follow-up to 0010 (ticket title is required; the category list
-- gains 'other' for requests not covered by the predefined categories).

ALTER TABLE tickets ADD COLUMN title TEXT NOT NULL DEFAULT '';

-- Backfill any pre-existing rows so the length CHECK below can be enforced.
UPDATE tickets SET title = 'Ticket' WHERE title = '';

ALTER TABLE tickets ALTER COLUMN title DROP DEFAULT;

ALTER TABLE tickets ADD CONSTRAINT tickets_title_length_check
    CHECK (char_length(title) BETWEEN 1 AND 120);

ALTER TABLE tickets DROP CONSTRAINT tickets_category_check;
ALTER TABLE tickets ADD CONSTRAINT tickets_category_check
    CHECK (category IN ('repair', 'noise_complaint', 'assembly_topic', 'other'));
