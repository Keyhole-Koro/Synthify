-- Document processing jobs & related infrastructure
--   document_processing_jobs : ジョブ本体
--   job_capabilities         : ジョブが操作できる範囲・上限
--   job_execution_plans      : 計画フェーズの出力
--   job_mutation_logs        : ジョブが行った全変更の記録
--   job_logs                 : 構造化ログ
--   job_approval_requests    : 承認待ち操作
--   job_stage_checkpoints    : ステージごとのチェックポイント (再開用)

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

CREATE INDEX IF NOT EXISTS idx_document_processing_jobs_document_id_created_at
  ON document_processing_jobs(document_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_document_processing_jobs_workspace_id_created_at
  ON document_processing_jobs(workspace_id, created_at DESC);

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

CREATE INDEX IF NOT EXISTS idx_job_capabilities_job_id ON job_capabilities(job_id);

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

CREATE INDEX IF NOT EXISTS idx_job_execution_plans_job_id ON job_execution_plans(job_id);

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

CREATE INDEX IF NOT EXISTS idx_job_mutation_logs_job_id ON job_mutation_logs(job_id);

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

CREATE INDEX IF NOT EXISTS idx_job_logs_job_id_created_at ON job_logs(job_id, created_at);
CREATE INDEX IF NOT EXISTS idx_job_logs_document_id_created_at ON job_logs(document_id, created_at);
CREATE INDEX IF NOT EXISTS idx_job_logs_workspace_id_created_at ON job_logs(workspace_id, created_at);
CREATE INDEX IF NOT EXISTS idx_job_logs_level_created_at ON job_logs(level, created_at);

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

CREATE TABLE IF NOT EXISTS job_stage_checkpoints (
  job_id     TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  stage      TEXT NOT NULL,
  status     TEXT NOT NULL,          -- running | succeeded | failed
  gcs_ref    TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (job_id, stage)
);

CREATE INDEX IF NOT EXISTS idx_job_stage_checkpoints_job_id ON job_stage_checkpoints(job_id);
