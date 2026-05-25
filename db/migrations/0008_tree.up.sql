-- Tree (workspace 内の知識構造)
--   tree_items     : ノード本体
--   item_sources   : ノードを支えるドキュメント由来の出典
--   item_aliases   : エイリアス正規化のステージング

CREATE TABLE IF NOT EXISTS tree_items (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  parent_id TEXT REFERENCES tree_items(id) ON DELETE SET NULL,
  title TEXT NOT NULL,
  level INTEGER NOT NULL DEFAULT 0,
  description TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  override_css TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  governance_state TEXT NOT NULL DEFAULT 'system_generated',
  last_mutation_job_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tree_items_workspace_id ON tree_items(workspace_id);
CREATE INDEX IF NOT EXISTS idx_tree_items_parent_id ON tree_items(parent_id);

CREATE TABLE IF NOT EXISTS item_sources (
  item_id TEXT NOT NULL REFERENCES tree_items(id) ON DELETE CASCADE,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  file_id TEXT NOT NULL REFERENCES document_files(file_id) ON DELETE CASCADE,
  chunk_id TEXT NOT NULL DEFAULT '',
  source_text TEXT NOT NULL DEFAULT '',
  confidence DOUBLE PRECISION,
  PRIMARY KEY (item_id, document_id, chunk_id)
);

CREATE INDEX IF NOT EXISTS idx_item_sources_item_id ON item_sources(item_id);

CREATE TABLE IF NOT EXISTS item_aliases (
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  canonical_item_id TEXT NOT NULL REFERENCES tree_items(id) ON DELETE CASCADE,
  alias_item_id TEXT NOT NULL REFERENCES tree_items(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending',
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (workspace_id, canonical_item_id, alias_item_id)
);
