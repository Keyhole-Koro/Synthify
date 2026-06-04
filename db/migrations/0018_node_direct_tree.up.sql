-- Node-direct tree model. See docs/improvements/tree-node-workspace-ownership.md.
--
-- The workspace itself is the tree root: knowledge nodes hang directly under
-- it (parent_id IS NULL) as a single integrated tree. Documents are no longer
-- tree nodes — they are sources, linked to nodes via item_sources. So this
-- migration removes the role-bearing scaffolding that 0008/0016 introduced:
--   - tree_items.kind (workspace_root / document_root / node) — roles are now
--     derived from parent_id, not a column.
--   - document_tree_links — documents no longer own a document_root node.
-- and adds:
--   - tree_items.cross_document — true when a node integrates more than one
--     source document.
--   - idx_tree_items_workspace_parent — supports "root nodes of a workspace"
--     lookups (WHERE workspace_id=? AND parent_id IS NULL).
--
-- Forward-only: 0008/0016 are left intact (they describe the schema as it was
-- when first applied); this migration evolves it from there.

ALTER TABLE tree_items
  ADD COLUMN IF NOT EXISTS cross_document BOOLEAN NOT NULL DEFAULT FALSE;

DROP INDEX IF EXISTS idx_tree_items_workspace_kind;

ALTER TABLE tree_items
  DROP COLUMN IF EXISTS kind;

CREATE INDEX IF NOT EXISTS idx_tree_items_workspace_parent
  ON tree_items(workspace_id, parent_id);

DROP TABLE IF EXISTS document_tree_links;
