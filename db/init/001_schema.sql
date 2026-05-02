CREATE TABLE IF NOT EXISTS accounts (
  account_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  plan TEXT NOT NULL,
  storage_quota_bytes BIGINT NOT NULL DEFAULT 0,
  storage_used_bytes BIGINT NOT NULL DEFAULT 0,
  max_file_size_bytes BIGINT NOT NULL DEFAULT 0,
  max_uploads_per_5h INTEGER NOT NULL DEFAULT 0,
  max_uploads_per_1week INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS account_users (
  account_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  user_id TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'member',
  joined_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (account_id, user_id)
);

CREATE TABLE IF NOT EXISTS workspaces (
  workspace_id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
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

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS document_chunks (
  chunk_id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  heading TEXT NOT NULL DEFAULT '',
  text TEXT NOT NULL,
  source_page INTEGER,
  embedding vector(768)
);

CREATE INDEX IF NOT EXISTS idx_document_chunks_document_id ON document_chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_document_chunks_embedding ON document_chunks USING hnsw (embedding vector_cosine_ops) WHERE embedding IS NOT NULL;

CREATE TABLE IF NOT EXISTS tree_items (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  parent_id TEXT REFERENCES tree_items(id) ON DELETE SET NULL,
  label TEXT NOT NULL,
  level INTEGER NOT NULL DEFAULT 0,
  description TEXT NOT NULL DEFAULT '',
  summary_html TEXT NOT NULL DEFAULT '',
  override_css TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  governance_state TEXT NOT NULL DEFAULT 'system_generated',
  last_mutation_job_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS document_processing_jobs (
  job_id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  job_type TEXT NOT NULL,
  status TEXT NOT NULL,
  current_stage TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  params_json TEXT NOT NULL DEFAULT '{}',
  requested_by TEXT NOT NULL DEFAULT '',
  capability_id TEXT NOT NULL DEFAULT '',
  execution_plan_id TEXT NOT NULL DEFAULT '',
  plan_status TEXT NOT NULL DEFAULT 'none',
  evaluation_status TEXT NOT NULL DEFAULT 'none',
  retry_count INTEGER NOT NULL DEFAULT 0,
  budget_json TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS item_sources (
  item_id TEXT NOT NULL REFERENCES tree_items(id) ON DELETE CASCADE,
  document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
  chunk_id TEXT NOT NULL DEFAULT '',
  source_text TEXT NOT NULL DEFAULT '',
  confidence DOUBLE PRECISION,
  PRIMARY KEY (item_id, document_id, chunk_id)
);

CREATE TABLE IF NOT EXISTS job_capabilities (
  capability_id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  allowed_document_ids_json TEXT NOT NULL DEFAULT '[]',
  allowed_item_ids_json TEXT NOT NULL DEFAULT '[]',
  allowed_operations_json TEXT NOT NULL DEFAULT '[]',
  max_llm_calls INTEGER NOT NULL DEFAULT 0,
  max_tool_runs INTEGER NOT NULL DEFAULT 0,
  max_item_creations INTEGER NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS job_execution_plans (
  plan_id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  summary TEXT NOT NULL,
  plan_json TEXT NOT NULL DEFAULT '{}',
  created_by TEXT NOT NULL DEFAULT 'planner',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS job_mutation_logs (
  mutation_id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  plan_id TEXT NOT NULL DEFAULT '',
  capability_id TEXT NOT NULL DEFAULT '',
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  mutation_type TEXT NOT NULL,
  risk_tier TEXT NOT NULL DEFAULT '',
  before_json TEXT NOT NULL DEFAULT '{}',
  after_json TEXT NOT NULL DEFAULT '{}',
  provenance_json TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS job_logs (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  document_id TEXT NOT NULL DEFAULT '',
  level TEXT NOT NULL,
  event TEXT NOT NULL,
  message TEXT NOT NULL,
  detail_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS job_approval_requests (
  approval_id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  plan_id TEXT NOT NULL REFERENCES job_execution_plans(plan_id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending',
  requested_operations_json TEXT NOT NULL DEFAULT '[]',
  reason TEXT NOT NULL DEFAULT '',
  risk_tier TEXT NOT NULL DEFAULT '',
  requested_by TEXT NOT NULL,
  reviewed_by TEXT NOT NULL DEFAULT '',
  requested_at TIMESTAMPTZ NOT NULL,
  reviewed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS item_aliases (
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  canonical_item_id TEXT NOT NULL REFERENCES tree_items(id) ON DELETE CASCADE,
  alias_item_id TEXT NOT NULL REFERENCES tree_items(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending',
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (workspace_id, canonical_item_id, alias_item_id)
);

CREATE INDEX IF NOT EXISTS idx_tree_items_workspace_id ON tree_items(workspace_id);
CREATE INDEX IF NOT EXISTS idx_tree_items_parent_id ON tree_items(parent_id);
CREATE INDEX IF NOT EXISTS idx_item_sources_item_id ON item_sources(item_id);
CREATE INDEX IF NOT EXISTS idx_job_capabilities_job_id ON job_capabilities(job_id);
CREATE INDEX IF NOT EXISTS idx_job_execution_plans_job_id ON job_execution_plans(job_id);
CREATE INDEX IF NOT EXISTS idx_job_mutation_logs_job_id ON job_mutation_logs(job_id);
CREATE INDEX IF NOT EXISTS idx_job_logs_job_id_created_at ON job_logs(job_id, created_at);
CREATE INDEX IF NOT EXISTS idx_job_logs_document_id_created_at ON job_logs(document_id, created_at);
CREATE INDEX IF NOT EXISTS idx_job_logs_workspace_id_created_at ON job_logs(workspace_id, created_at);
CREATE INDEX IF NOT EXISTS idx_job_logs_level_created_at ON job_logs(level, created_at);
