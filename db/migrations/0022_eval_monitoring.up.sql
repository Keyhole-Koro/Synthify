-- Persist LLM eval results for the monitor dashboard.
--
-- eval writes through the application DSN. monitor reads only through views.
--
-- Views and GRANT/REVOKE are intentionally NOT here — they live in
-- 0024_eval_views_grants. CockroachDB rejects a migration that creates a table
-- and, in the same transaction, creates a view depending on it or grants on it
-- (golang-migrate runs each file as one transaction). This file therefore does
-- DDL only (matching the working 0007_jobs pattern); 0024 does the dependent
-- views + grants (matching the working 0014_monitor_role pattern).

CREATE TABLE IF NOT EXISTS eval_runs (
  run_id UUID PRIMARY KEY,
  prompt_source STRING NOT NULL,
  artifact_uri STRING NOT NULL DEFAULT '',
  status STRING NOT NULL,
  case_count INT8 NOT NULL,
  passed_count INT8 NOT NULL,
  failed_count INT8 NOT NULL,
  pass_rate FLOAT8 NOT NULL,
  duration_ms INT8 NOT NULL,
  model STRING NOT NULL DEFAULT '',
  input_tokens INT8 NOT NULL DEFAULT 0,
  output_tokens INT8 NOT NULL DEFAULT 0,
  started_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT eval_runs_status_check CHECK (status IN ('succeeded', 'failed'))
);

CREATE INDEX IF NOT EXISTS eval_runs_started_at_idx
  ON eval_runs (started_at DESC);
CREATE INDEX IF NOT EXISTS eval_runs_prompt_source_started_at_idx
  ON eval_runs (prompt_source, started_at DESC);

CREATE TABLE IF NOT EXISTS eval_case_results (
  run_id UUID NOT NULL REFERENCES eval_runs (run_id) ON DELETE CASCADE,
  case_index INT8 NOT NULL,
  case_name STRING NOT NULL,
  tool STRING NOT NULL,
  passed BOOL NOT NULL,
  schema_valid BOOL NOT NULL,
  duration_ms INT8 NOT NULL,
  model STRING NOT NULL DEFAULT '',
  input_tokens INT8 NOT NULL DEFAULT 0,
  output_tokens INT8 NOT NULL DEFAULT 0,
  error STRING NOT NULL DEFAULT '',
  output_json JSONB NULL,
  failed_input_json JSONB NULL,
  prompt_source STRING NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, case_index)
);

CREATE INDEX IF NOT EXISTS eval_case_results_created_at_idx
  ON eval_case_results (created_at DESC);
CREATE INDEX IF NOT EXISTS eval_case_results_failed_idx
  ON eval_case_results (passed, created_at DESC);
CREATE INDEX IF NOT EXISTS eval_case_results_model_idx
  ON eval_case_results (model, created_at DESC);
