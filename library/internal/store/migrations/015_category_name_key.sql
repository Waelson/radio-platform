-- Add normalized name_key to categories for deduplication-safe lookups.
-- Populated from existing name rows; managed by application on insert/update.
ALTER TABLE categories ADD COLUMN name_key TEXT NOT NULL DEFAULT '';
UPDATE categories SET name_key = LOWER(TRIM(name));
CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_name_key ON categories(name_key);
