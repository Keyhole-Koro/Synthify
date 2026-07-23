-- name: InsertEvalRun :exec
INSERT INTO eval_runs (
  run_id, prompt_source, artifact_uri, status,
  case_count, passed_count, failed_count, pass_rate,
  duration_ms, model, input_tokens, output_tokens,
  started_at, completed_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: InsertEvalCaseResult :exec
INSERT INTO eval_case_results (
  run_id, case_index, case_name, tool, passed, schema_valid,
  duration_ms, model, input_tokens, output_tokens, error,
  output_json, failed_input_json, prompt_source
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: InsertEvalTraceEvent :exec
INSERT INTO eval_trace_events (
  run_id, case_index, event_id, parent_event_id, sequence, kind, name,
  started_at, completed_at, duration_ms, model, input_tokens, output_tokens,
  input_json, output_json, error
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16);
