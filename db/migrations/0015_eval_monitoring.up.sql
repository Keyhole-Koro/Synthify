-- Persist LLM eval results for the monitor dashboard.
--
-- eval writes through the application DSN. monitor reads only through views.

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

CREATE OR REPLACE VIEW v_eval_runs AS
  SELECT
    run_id,
    prompt_source,
    artifact_uri,
    status,
    case_count,
    passed_count,
    failed_count,
    pass_rate,
    duration_ms,
    model,
    input_tokens,
    output_tokens,
    started_at,
    completed_at,
    created_at
  FROM eval_runs;

CREATE OR REPLACE VIEW v_eval_case_results AS
  SELECT
    run_id,
    case_index,
    case_name,
    tool,
    passed,
    schema_valid,
    duration_ms,
    model,
    input_tokens,
    output_tokens,
    error,
    output_json,
    failed_input_json,
    prompt_source,
    created_at
  FROM eval_case_results;

GRANT SELECT ON eval_runs TO monitor;
GRANT SELECT ON eval_case_results TO monitor;
GRANT SELECT ON v_eval_runs TO monitor;
GRANT SELECT ON v_eval_case_results TO monitor;

REVOKE INSERT, UPDATE, DELETE ON eval_runs FROM monitor;
REVOKE INSERT, UPDATE, DELETE ON eval_case_results FROM monitor;
