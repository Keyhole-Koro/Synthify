-- Per-case LLM/tool execution trace spans.
--
-- DDL only. The v_eval_trace_events view and its grants live in
-- 0024_eval_views_grants, so CockroachDB does not have to create a table and a
-- dependent view/grant in the same migration transaction (see 0022).

CREATE TABLE IF NOT EXISTS eval_trace_events (
  run_id UUID NOT NULL REFERENCES eval_runs (run_id) ON DELETE CASCADE,
  case_index INT8 NOT NULL,
  event_id UUID NOT NULL,
  parent_event_id UUID NULL,
  sequence INT8 NOT NULL,
  kind STRING NOT NULL,
  name STRING NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ NOT NULL,
  duration_ms INT8 NOT NULL,
  model STRING NOT NULL DEFAULT '',
  input_tokens INT8 NOT NULL DEFAULT 0,
  output_tokens INT8 NOT NULL DEFAULT 0,
  input_json JSONB NULL,
  output_json JSONB NULL,
  error STRING NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, case_index, event_id),
  CONSTRAINT eval_trace_events_kind_check CHECK (kind IN ('tool', 'llm', 'validation', 'assertion')),
  CONSTRAINT eval_trace_events_case_fk FOREIGN KEY (run_id, case_index)
    REFERENCES eval_case_results (run_id, case_index) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS eval_trace_events_case_sequence_idx
  ON eval_trace_events (run_id, case_index, sequence);
CREATE INDEX IF NOT EXISTS eval_trace_events_created_at_idx
  ON eval_trace_events (created_at DESC);
