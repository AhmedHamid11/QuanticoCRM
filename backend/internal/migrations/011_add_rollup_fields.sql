-- Migration: Add rollup field support
-- Stores SQL query and result type configuration for rollup fields

ALTER TABLE field_defs ADD COLUMN rollup_query TEXT;
ALTER TABLE field_defs ADD COLUMN rollup_result_type TEXT; -- 'numeric' or 'text'
ALTER TABLE field_defs ADD COLUMN rollup_decimal_places INTEGER DEFAULT 2;
