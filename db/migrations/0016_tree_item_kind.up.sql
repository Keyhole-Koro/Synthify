-- tree_items.kind: makes the "what role does this item play" question a
-- first-class lookup instead of the frontend's findRootItemId heuristic.
--
-- Values:
--   workspace_root  exactly one per workspace; parent_id IS NULL
--   document_root   one per document; parent_id = the workspace_root
--   node            everything else
--
-- Defaults to 'node' for legacy rows. The worker is updated in PR 3 to
-- emit the right kind when it creates new workspace roots and document
-- roots. Backfill is intentionally omitted: prod is empty, stage data
-- can be reset.

ALTER TABLE tree_items
  ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'node'
    CHECK (kind IN ('workspace_root', 'document_root', 'node'));

CREATE INDEX IF NOT EXISTS idx_tree_items_workspace_kind
  ON tree_items(workspace_id, kind);

-- document_tree_links: enforces document <-> document_root_item as a 1:1
-- via UNIQUE on both sides. Without this the relationship lives only in
-- item_sources (which is N:N evidence linkage) and the frontend has to
-- guess via "first child of the workspace root".

CREATE TABLE IF NOT EXISTS document_tree_links (
  document_id  TEXT NOT NULL UNIQUE REFERENCES documents(document_id) ON DELETE CASCADE,
  root_item_id TEXT NOT NULL UNIQUE REFERENCES tree_items(id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_document_tree_links_workspace_id
  ON document_tree_links(workspace_id);
