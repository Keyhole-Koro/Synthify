-- Reverting drops the reconciled objects. Harmless where 0008 is the real
-- owner of the schema; the canonical definition lives in 0008_tree.up.sql.
DROP TABLE IF EXISTS document_tree_links;
DROP INDEX IF EXISTS idx_tree_items_workspace_kind;
ALTER TABLE tree_items DROP COLUMN IF EXISTS kind;
