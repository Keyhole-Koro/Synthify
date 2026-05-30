-- Reconcile tree_items.kind + document_tree_links onto databases that were
-- force-baselined past the original 0016_tree_item_kind migration.
--
-- History: 0016_tree_item_kind introduced these objects, then a later commit
-- deleted that file and merged its DDL into 0008_tree.up.sql in place, on the
-- assumption that every environment would be reset. prod was NOT reset: it was
-- created from db/init and stamped to schema_migrations version 16 via
-- `migrate force` without that SQL ever running, so it has neither the kind
-- column nor document_tree_links. CreateWorkspace then fails with
-- `column "kind" does not exist` (42703). stage applied the real 0016 and
-- already has both.
--
-- Every statement is idempotent, so this is a no-op on stage and the actual
-- fix on prod. kind is NOT NULL with no default to match the canonical 0008
-- definition; that is safe because the only database missing it (prod) has zero
-- tree_items rows.
ALTER TABLE tree_items
  ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL
  CHECK (kind IN ('workspace_root', 'document_root', 'node'));

CREATE INDEX IF NOT EXISTS idx_tree_items_workspace_kind ON tree_items(workspace_id, kind);

CREATE TABLE IF NOT EXISTS document_tree_links (
  document_id  TEXT NOT NULL UNIQUE REFERENCES documents(document_id) ON DELETE CASCADE,
  root_item_id TEXT NOT NULL UNIQUE REFERENCES tree_items(id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_document_tree_links_workspace_id
  ON document_tree_links(workspace_id);
