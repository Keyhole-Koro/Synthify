-- Views + monitor grants for the eval tables created in 0022 (eval_runs,
-- eval_case_results) and 0023 (eval_trace_events).
--
-- Split out of 0022/0023 on purpose: CockroachDB rejects creating a table and,
-- in the same migration transaction, a view depending on it or a GRANT on it.
-- Here the referenced tables are already committed, so this file only creates
-- views and adjusts privileges — the same shape as the working 0014_monitor_role
-- migration (grant on existing tables + create view + grant on view).
--
-- monitor reads eval data only through these views (0014 creates the role).

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

GRANT SELECT ON eval_runs TO monitor;
GRANT SELECT ON eval_case_results TO monitor;
GRANT SELECT ON eval_trace_events TO monitor;
GRANT SELECT ON v_eval_runs TO monitor;
GRANT SELECT ON v_eval_case_results TO monitor;
GRANT SELECT ON v_eval_trace_events TO monitor;

REVOKE INSERT, UPDATE, DELETE ON eval_runs FROM monitor;
REVOKE INSERT, UPDATE, DELETE ON eval_case_results FROM monitor;
REVOKE INSERT, UPDATE, DELETE ON eval_trace_events FROM monitor;
