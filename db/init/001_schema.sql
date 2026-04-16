CREATE TABLE IF NOT EXISTS accounts (
  account_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  plan TEXT NOT NULL,
  storage_quota_bytes BIGINT NOT NULL DEFAULT 0,
  storage_used_bytes BIGINT NOT NULL DEFAULT 0,
  max_file_size_bytes BIGINT NOT NULL DEFAULT 0,
  max_uploads_per_5h BIGINT NOT NULL DEFAULT 0,
  max_uploads_per_1week BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS account_users (
  account_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  user_id TEXT NOT NULL,
  role TEXT NOT NULL,
  joined_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (account_id, user_id)
);

CREATE TABLE IF NOT EXISTS workspaces (
  workspace_id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS documents (
  document_id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  uploaded_by TEXT NOT NULL,
  filename TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  file_size BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS document_chunks (
  chunk_id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  heading TEXT NOT NULL DEFAULT '',
  text TEXT NOT NULL,
  source_page INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS document_processing_jobs (
  job_id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  graph_id TEXT,
  job_type TEXT NOT NULL,
  status TEXT NOT NULL,
  current_stage TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  params_json TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS graphs (
  graph_id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL UNIQUE REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS nodes (
  node_id TEXT PRIMARY KEY,
  graph_id TEXT NOT NULL REFERENCES graphs(graph_id) ON DELETE CASCADE,
  label TEXT NOT NULL,
  category TEXT NOT NULL DEFAULT '',
  entity_type TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  summary_html TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS edges (
  edge_id TEXT PRIMARY KEY,
  graph_id TEXT NOT NULL REFERENCES graphs(graph_id) ON DELETE CASCADE,
  source_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  target_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  edge_type TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS node_sources (
  node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  chunk_id TEXT NOT NULL DEFAULT '',
  source_text TEXT NOT NULL DEFAULT '',
  confidence DOUBLE PRECISION,
  PRIMARY KEY (node_id, document_id, chunk_id)
);

CREATE TABLE IF NOT EXISTS edge_sources (
  edge_id TEXT NOT NULL REFERENCES edges(edge_id) ON DELETE CASCADE,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  chunk_id TEXT NOT NULL DEFAULT '',
  source_text TEXT NOT NULL DEFAULT '',
  confidence DOUBLE PRECISION,
  PRIMARY KEY (edge_id, document_id, chunk_id)
);

CREATE TABLE IF NOT EXISTS node_aliases (
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  canonical_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  alias_node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (workspace_id, canonical_node_id, alias_node_id)
);

CREATE INDEX IF NOT EXISTS idx_account_users_user_id ON account_users(user_id);
CREATE INDEX IF NOT EXISTS idx_workspaces_account_id ON workspaces(account_id);
CREATE INDEX IF NOT EXISTS idx_documents_workspace_id ON documents(workspace_id);
CREATE INDEX IF NOT EXISTS idx_document_chunks_document_id ON document_chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_document_processing_jobs_document_id ON document_processing_jobs(document_id);
CREATE INDEX IF NOT EXISTS idx_nodes_graph_id ON nodes(graph_id);
CREATE INDEX IF NOT EXISTS idx_edges_graph_id ON edges(graph_id);
CREATE INDEX IF NOT EXISTS idx_node_sources_document_id ON node_sources(document_id);
CREATE INDEX IF NOT EXISTS idx_edge_sources_document_id ON edge_sources(document_id);
