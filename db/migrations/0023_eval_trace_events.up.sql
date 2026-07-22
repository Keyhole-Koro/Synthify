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

CREATE OR REPLACE VIEW v_eval_trace_events AS
  SELECT
    e.run_id,
    e.case_index,
    c.case_name,
    e.event_id,
    e.parent_event_id,
    e.sequence,
    e.kind,
    e.name,
    e.started_at,
    e.completed_at,
    e.duration_ms,
    e.model,
    e.input_tokens,
    e.output_tokens,
    e.input_json,
    e.output_json,
    e.error,
    e.created_at
  FROM eval_trace_events e
  JOIN eval_case_results c
    ON c.run_id = e.run_id AND c.case_index = e.case_index;

GRANT SELECT ON eval_trace_events TO monitor;
GRANT SELECT ON v_eval_trace_events TO monitor;
REVOKE INSERT, UPDATE, DELETE ON eval_trace_events FROM monitor;
