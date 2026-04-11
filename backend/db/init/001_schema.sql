CREATE TABLE IF NOT EXISTS workspaces (
  workspace_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  owner_id TEXT NOT NULL,
  plan TEXT NOT NULL,
  storage_used_bytes BIGINT NOT NULL DEFAULT 0,
  storage_quota_bytes BIGINT NOT NULL DEFAULT 0,
  max_file_size_bytes BIGINT NOT NULL DEFAULT 0,
  max_uploads_per_day BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS workspace_members (
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  user_id TEXT NOT NULL,
  email TEXT NOT NULL,
  role TEXT NOT NULL,
  is_dev BOOLEAN NOT NULL DEFAULT FALSE,
  invited_at TIMESTAMPTZ NOT NULL,
  invited_by TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE IF NOT EXISTS documents (
  document_id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  uploaded_by TEXT NOT NULL,
  filename TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  file_size BIGINT NOT NULL,
  status TEXT NOT NULL,
  extraction_depth TEXT NOT NULL DEFAULT '',
  node_count INTEGER NOT NULL DEFAULT 0,
  current_stage TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS nodes (
  node_id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  label TEXT NOT NULL,
  level INTEGER NOT NULL,
  category TEXT NOT NULL,
  entity_type TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  summary_html TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS edges (
  edge_id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  source_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  target_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  edge_type TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS node_views (
  id BIGSERIAL PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  user_id TEXT NOT NULL,
  node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  viewed_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS node_aliases (
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  canonical_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  alias_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (workspace_id, canonical_node_id, alias_node_id)
);

CREATE INDEX IF NOT EXISTS idx_documents_workspace_id ON documents(workspace_id);
CREATE INDEX IF NOT EXISTS idx_nodes_document_id ON nodes(document_id);
CREATE INDEX IF NOT EXISTS idx_edges_document_id ON edges(document_id);
CREATE INDEX IF NOT EXISTS idx_node_views_workspace_user ON node_views(workspace_id, user_id, viewed_at DESC);
