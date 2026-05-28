DROP INDEX IF EXISTS idx_document_tree_links_workspace_id;
DROP TABLE IF EXISTS document_tree_links;
DROP INDEX IF EXISTS idx_tree_items_workspace_kind;
ALTER TABLE tree_items DROP COLUMN IF EXISTS kind;
