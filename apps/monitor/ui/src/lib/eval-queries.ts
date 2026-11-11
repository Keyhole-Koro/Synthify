/**
 * PostgreSQL analytical queries for the Evaluation dashboard, benchmarking metrics,
 * and per-case execution traces.
 *
 * @module
 */
import type { Pool } from 'pg';
import type { Period } from './dashboard-queries';

/** Formats SQL interval string for the selected period. */
function periodInterval(period: Period): string {
  switch (period) {
    case 'today': return "date_trunc('day', NOW())";
    case '7d': return "NOW() - INTERVAL '7 days'";
    case '30d': return "NOW() - INTERVAL '30 days'";
  }
}

/**
 * Point in time for evaluation pass rates and run trends.
 * @property date ISO date or hour bucket.
 * @property runs Number of evaluation runs in the bucket.
 * @property cases Number of evaluation cases in the bucket.
 * @property passed Number of passing cases in the bucket.
 * @property passRate Passing cases divided by total cases, from 0 to 1.
 */
export interface EvalTrendPoint {
  date: string;
  runs: number;
  cases: number;
  passed: number;
  passRate: number;
}

/**
 * Performance and accuracy metrics grouped by prompt source variant.
 * @property promptSource Prompt or instruction source identifier.
 * @property runs Number of runs using the source.
 * @property cases Number of cases evaluated.
 * @property passRate Passing cases divided by total cases, from 0 to 1.
 * @property avgDurationMs Average case duration in milliseconds.
 * @property inputTokens Total input tokens consumed.
 * @property outputTokens Total output tokens generated.
 */
export interface EvalPromptSourceRow {
  promptSource: string;
  runs: number;
  cases: number;
  passRate: number;
  avgDurationMs: number;
  inputTokens: number;
  outputTokens: number;
}

/**
 * Performance and accuracy metrics grouped by LLM model.
 * @property model Model identifier.
 * @property cases Number of cases evaluated by the model.
 * @property passRate Passing cases divided by total cases, from 0 to 1.
 * @property avgDurationMs Average case duration in milliseconds.
 * @property p95DurationMs 95th-percentile case duration in milliseconds.
 * @property inputTokens Total input tokens consumed.
 * @property outputTokens Total output tokens generated.
 */
export interface EvalModelRow {
  model: string;
  cases: number;
  passRate: number;
  avgDurationMs: number;
  p95DurationMs: number;
  inputTokens: number;
  outputTokens: number;
}

/**
 * Individual evaluation run summary record.
 * @property runId Evaluation run identifier.
 * @property promptSource Prompt or instruction source identifier.
 * @property status Whether the run completed successfully or failed.
 * @property caseCount Number of cases in the run.
 * @property passedCount Number of passing cases.
 * @property failedCount Number of failing cases.
 * @property passRate Passing cases divided by total cases, from 0 to 1.
 * @property durationMs Total run duration in milliseconds.
 * @property model Model used for the run.
 * @property inputTokens Total input tokens consumed.
 * @property outputTokens Total output tokens generated.
 * @property artifactUri Optional URI for persisted evaluation artifacts.
 * @property startedAt ISO-8601 run start timestamp.
 * @property completedAt ISO-8601 run completion timestamp.
 */
export interface EvalRunRow {
  runId: string;
  promptSource: string;
  status: 'succeeded' | 'failed';
  caseCount: number;
  passedCount: number;
  failedCount: number;
  passRate: number;
  durationMs: number;
  model: string;
  inputTokens: number;
  outputTokens: number;
  artifactUri: string;
  startedAt: string;
  completedAt: string;
}

/**
 * Evaluation case execution result row.
 * @property runId Parent evaluation run identifier.
 * @property caseIndex Zero-based case position in the run.
 * @property caseName Human-readable case name.
 * @property tool Tool exercised by the case.
 * @property passed Whether the case assertions passed.
 * @property schemaValid Whether the model output matched the expected schema.
 * @property model Model used for the case.
 * @property durationMs Case duration in milliseconds.
 * @property inputTokens Input tokens consumed.
 * @property outputTokens Output tokens generated.
 * @property error Execution or assertion error message, when present.
 * @property output Parsed case output, when available.
 * @property failedInput Input captured for a failed case, when available.
 * @property promptSource Prompt or instruction source identifier.
 * @property createdAt ISO-8601 case creation timestamp.
 */
export interface EvalCaseRow {
  runId: string;
  caseIndex: number;
  caseName: string;
  tool: string;
  passed: boolean;
  schemaValid: boolean;
  model: string;
  durationMs: number;
  inputTokens: number;
  outputTokens: number;
  error: string;
  output: unknown;
  failedInput: unknown;
  promptSource: string;
  createdAt: string;
}

/**
 * Funnel stage metrics tracking execution, schema validation, and assertion passes.
 * @property totalCases Total cases considered by the funnel.
 * @property executionCompleted Cases whose execution completed.
 * @property schemaValid Cases with schema-valid output.
 * @property passed Cases passing all assertions.
 * @property executionErrors Cases failing during execution.
 * @property schemaInvalid Cases producing schema-invalid output.
 * @property assertionFailures Cases failing assertions after execution.
 */
export interface EvalFunnelData {
  totalCases: number;
  executionCompleted: number;
  schemaValid: number;
  passed: number;
  executionErrors: number;
  schemaInvalid: number;
  assertionFailures: number;
}

/**
 * Full evaluation dashboard payload containing metrics, funnels, and recent cases.
 * @property totalRuns Number of evaluation runs in the period.
 * @property totalCases Number of evaluation cases in the period.
 * @property passRate Overall passing-case ratio, from 0 to 1.
 * @property avgRunDurationMs Average run duration in milliseconds.
 * @property p95CaseDurationMs 95th-percentile case duration in milliseconds.
 * @property inputTokens Total input tokens consumed.
 * @property outputTokens Total output tokens generated.
 * @property funnel Funnel counts across execution and validation stages.
 * @property trend Time-series pass-rate points.
 * @property byPromptSource Metrics grouped by prompt source.
 * @property byModel Metrics grouped by model.
 * @property recentRuns Most recent run summaries.
 * @property recentCases Most recent case results.
 * @property recentFailures Most recent failed cases.
 * @property slowestCases Cases with the longest execution duration.
 */
export interface EvalData {
  totalRuns: number;
  totalCases: number;
  passRate: number;
  avgRunDurationMs: number;
  p95CaseDurationMs: number;
  inputTokens: number;
  outputTokens: number;
  funnel: EvalFunnelData;
  trend: EvalTrendPoint[];
  byPromptSource: EvalPromptSourceRow[];
  byModel: EvalModelRow[];
  recentRuns: EvalRunRow[];
  recentCases: EvalCaseRow[];
  recentFailures: EvalCaseRow[];
  slowestCases: EvalCaseRow[];
}

/**
 * Queries evaluation dashboard metrics, trends, model comparisons, and recent case results.
 *
 * @param pool PostgreSQL client pool.
 * @param period Lookback time period.
 * @returns Promise resolving to EvalData.
 */
export async function queryEval(pool: Pool, period: Period): Promise<EvalData> {
  const since = periodInterval(period);
  const dateTrunc = period === 'today' ? 'hour' : 'day';
  const dateLabel = period === 'today'
    ? `to_char(date_trunc('hour', started_at), 'HH24:00')`
    : `date_trunc('day', started_at)::date::text`;

  const [overview, caseOverview, trend, byPromptSource, byModel, recentRuns, recentCases, recentFailures, slowestCases] = await Promise.all([
    pool.query(`
      SELECT
        COUNT(*) AS total_runs,
        COALESCE(SUM(case_count), 0) AS total_cases,
        COALESCE(SUM(passed_count), 0) AS passed_cases,
        COALESCE(ROUND(AVG(duration_ms)), 0) AS avg_run_duration_ms,
        COALESCE(SUM(input_tokens), 0) AS input_tokens,
        COALESCE(SUM(output_tokens), 0) AS output_tokens
      FROM v_eval_runs
      WHERE started_at >= ${since}
    `),
    pool.query(`
      SELECT
        COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms), 0) AS p95_case_duration_ms,
        COUNT(*) AS total_cases,
        COUNT(*) FILTER (WHERE error = '') AS execution_completed,
        COUNT(*) FILTER (WHERE schema_valid) AS schema_valid,
        COUNT(*) FILTER (WHERE passed) AS passed,
        COUNT(*) FILTER (WHERE error <> '') AS execution_errors,
        COUNT(*) FILTER (WHERE error = '' AND NOT schema_valid) AS schema_invalid,
        COUNT(*) FILTER (WHERE error = '' AND schema_valid AND NOT passed) AS assertion_failures
      FROM v_eval_case_results
      WHERE created_at >= ${since}
    `),
    pool.query(`
      SELECT
        ${dateLabel} AS date,
        COUNT(*) AS runs,
        COALESCE(SUM(case_count), 0) AS cases,
        COALESCE(SUM(passed_count), 0) AS passed
      FROM v_eval_runs
      WHERE started_at >= ${since}
      GROUP BY date_trunc('${dateTrunc}', started_at)
      ORDER BY date_trunc('${dateTrunc}', started_at)
    `),
    pool.query(`
      SELECT
        prompt_source,
        COUNT(*) AS runs,
        COALESCE(SUM(case_count), 0) AS cases,
        COALESCE(SUM(passed_count), 0) AS passed,
        COALESCE(ROUND(AVG(duration_ms)), 0) AS avg_duration_ms,
        COALESCE(SUM(input_tokens), 0) AS input_tokens,
        COALESCE(SUM(output_tokens), 0) AS output_tokens
      FROM v_eval_runs
      WHERE started_at >= ${since}
      GROUP BY prompt_source
      ORDER BY cases DESC
    `),
    pool.query(`
      SELECT
        CASE WHEN model = '' THEN 'unknown' ELSE model END AS model,
        COUNT(*) AS cases,
        COUNT(*) FILTER (WHERE passed) AS passed,
        COALESCE(ROUND(AVG(duration_ms)), 0) AS avg_duration_ms,
        COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms), 0) AS p95_duration_ms,
        COALESCE(SUM(input_tokens), 0) AS input_tokens,
        COALESCE(SUM(output_tokens), 0) AS output_tokens
      FROM v_eval_case_results
      WHERE created_at >= ${since}
      GROUP BY CASE WHEN model = '' THEN 'unknown' ELSE model END
      ORDER BY cases DESC
    `),
    pool.query(`
      SELECT
        run_id,
        prompt_source,
        status,
        case_count,
        passed_count,
        failed_count,
        pass_rate,
        duration_ms,
        model,
        input_tokens,
        output_tokens,
        artifact_uri,
        started_at,
        completed_at
      FROM v_eval_runs
      WHERE started_at >= ${since}
      ORDER BY started_at DESC
      LIMIT 25
    `),
    pool.query(`
      SELECT
        run_id,
        case_index,
        case_name,
        tool,
        passed,
        schema_valid,
        model,
        duration_ms,
        input_tokens,
        output_tokens,
        error,
        output_json,
        failed_input_json,
        prompt_source,
        created_at
      FROM v_eval_case_results
      WHERE created_at >= ${since}
      ORDER BY created_at DESC
      LIMIT 50
    `),
    pool.query(`
      SELECT
        run_id,
        case_index,
        case_name,
        tool,
        passed,
        schema_valid,
        model,
        duration_ms,
        input_tokens,
        output_tokens,
        error,
        output_json,
        failed_input_json,
        prompt_source,
        created_at
      FROM v_eval_case_results
      WHERE NOT passed AND created_at >= ${since}
      ORDER BY created_at DESC
      LIMIT 25
    `),
    pool.query(`
      SELECT
        run_id,
        case_index,
        case_name,
        tool,
        passed,
        schema_valid,
        model,
        duration_ms,
        input_tokens,
        output_tokens,
        error,
        output_json,
        failed_input_json,
        prompt_source,
        created_at
      FROM v_eval_case_results
      WHERE created_at >= ${since}
      ORDER BY duration_ms DESC
      LIMIT 15
    `),
  ]);

  const ov = overview.rows[0] ?? {};
  const caseOv = caseOverview.rows[0] ?? {};
  const totalCases = Number(ov.total_cases ?? 0);
  const passedCases = Number(ov.passed_cases ?? 0);
  const toISO = (value: unknown) => value instanceof Date ? value.toISOString() : String(value ?? '');
  const caseRow = (r: Record<string, unknown>): EvalCaseRow => ({
    runId: String(r.run_id),
    caseIndex: Number(r.case_index ?? 0),
    caseName: String(r.case_name),
    tool: String(r.tool),
    passed: Boolean(r.passed),
    schemaValid: Boolean(r.schema_valid),
    model: String(r.model || 'unknown'),
    durationMs: Number(r.duration_ms),
    inputTokens: Number(r.input_tokens ?? 0),
    outputTokens: Number(r.output_tokens ?? 0),
    error: String(r.error || ''),
    output: r.output_json ?? null,
    failedInput: r.failed_input_json ?? null,
    promptSource: String(r.prompt_source),
    createdAt: toISO(r.created_at),
  });

  return {
    totalRuns: Number(ov.total_runs ?? 0),
    totalCases,
    passRate: totalCases > 0 ? passedCases / totalCases : 0,
    avgRunDurationMs: Number(ov.avg_run_duration_ms ?? 0),
    p95CaseDurationMs: Number(caseOv.p95_case_duration_ms ?? 0),
    inputTokens: Number(ov.input_tokens ?? 0),
    outputTokens: Number(ov.output_tokens ?? 0),
    funnel: {
      totalCases: Number(caseOv.total_cases ?? totalCases),
      executionCompleted: Number(caseOv.execution_completed ?? 0),
      schemaValid: Number(caseOv.schema_valid ?? 0),
      passed: Number(caseOv.passed ?? passedCases),
      executionErrors: Number(caseOv.execution_errors ?? 0),
      schemaInvalid: Number(caseOv.schema_invalid ?? 0),
      assertionFailures: Number(caseOv.assertion_failures ?? 0),
    },
    trend: trend.rows.map((r) => {
      const cases = Number(r.cases ?? 0);
      const passed = Number(r.passed ?? 0);
      return {
        date: String(r.date),
        runs: Number(r.runs),
        cases,
        passed,
        passRate: cases > 0 ? passed / cases : 0,
      };
    }),
    byPromptSource: byPromptSource.rows.map((r) => {
      const cases = Number(r.cases ?? 0);
      const passed = Number(r.passed ?? 0);
      return {
        promptSource: String(r.prompt_source),
        runs: Number(r.runs),
        cases,
        passRate: cases > 0 ? passed / cases : 0,
        avgDurationMs: Number(r.avg_duration_ms ?? 0),
        inputTokens: Number(r.input_tokens ?? 0),
        outputTokens: Number(r.output_tokens ?? 0),
      };
    }),
    byModel: byModel.rows.map((r) => {
      const cases = Number(r.cases ?? 0);
      const passed = Number(r.passed ?? 0);
      return {
        model: String(r.model),
        cases,
        passRate: cases > 0 ? passed / cases : 0,
        avgDurationMs: Number(r.avg_duration_ms ?? 0),
        p95DurationMs: Number(r.p95_duration_ms ?? 0),
        inputTokens: Number(r.input_tokens ?? 0),
        outputTokens: Number(r.output_tokens ?? 0),
      };
    }),
    recentRuns: recentRuns.rows.map((r) => ({
      runId: String(r.run_id),
      promptSource: String(r.prompt_source),
      status: r.status === 'succeeded' ? 'succeeded' : 'failed',
      caseCount: Number(r.case_count),
      passedCount: Number(r.passed_count),
      failedCount: Number(r.failed_count),
      passRate: Number(r.pass_rate),
      durationMs: Number(r.duration_ms),
      model: String(r.model || 'unknown'),
      inputTokens: Number(r.input_tokens),
      outputTokens: Number(r.output_tokens),
      artifactUri: String(r.artifact_uri || ''),
      startedAt: toISO(r.started_at),
      completedAt: toISO(r.completed_at),
    })),
    recentCases: recentCases.rows.map(caseRow),
    recentFailures: recentFailures.rows.map(caseRow),
    slowestCases: slowestCases.rows.map(caseRow),
  };
}

// ── Eval execution trace ────────────────────────────────────────────────────

/**
 * Individual ordered execution span event (tool, LLM call, validation, or assertion)
 * persisted during an evaluation case run.
 */
export interface EvalTraceEvent {
  /** Trace event identifier. */
  eventId: string;
  /** Parent event identifier, or an empty value for a root event. */
  parentEventId: string;
  /** Monotonic event sequence within the trace. */
  sequence: number;
  /** Operation category represented by the event. */
  kind: 'tool' | 'llm' | 'validation' | 'assertion';
  /** Human-readable operation name. */
  name: string;
  /** ISO-8601 event start timestamp. */
  startedAt: string;
  /** ISO-8601 event completion timestamp. */
  completedAt: string;
  /** Event duration in milliseconds. */
  durationMs: number;
  /** Model involved in the event, when applicable. */
  model: string;
  /** Input tokens consumed, when applicable. */
  inputTokens: number;
  /** Output tokens generated, when applicable. */
  outputTokens: number;
  /** Input payload captured for the event. */
  input: unknown;
  /** Output payload captured for the event. */
  output: unknown;
  /** Error message, or an empty string when the event succeeded. */
  error: string;
}

/**
 * Queries the ordered execution trace events recorded for a specific case in an evaluation run.
 *
 * @param pool PostgreSQL client pool.
 * @param runId Evaluation run UUID.
 * @param caseIndex Numeric index of the evaluated case within the run.
 * @returns Promise resolving to an ordered list of EvalTraceEvent spans.
 */
export async function queryEvalCaseTrace(
  pool: Pool,
  runId: string,
  caseIndex: number,
): Promise<EvalTraceEvent[]> {
  const { rows } = await pool.query(
    `SELECT
       event_id, parent_event_id, sequence, kind, name,
       started_at, completed_at, duration_ms, model,
       input_tokens, output_tokens, input_json, output_json, error
     FROM v_eval_trace_events
     WHERE run_id = $1 AND case_index = $2
     ORDER BY sequence ASC`,
    [runId, caseIndex],
  );
  const toISO = (value: unknown) => (value instanceof Date ? value.toISOString() : String(value ?? ''));
  return rows.map((r) => ({
    eventId: String(r.event_id),
    parentEventId: r.parent_event_id ? String(r.parent_event_id) : '',
    sequence: Number(r.sequence ?? 0),
    kind: (String(r.kind) as EvalTraceEvent['kind']),
    name: String(r.name ?? ''),
    startedAt: toISO(r.started_at),
    completedAt: toISO(r.completed_at),
    durationMs: Number(r.duration_ms ?? 0),
    model: String(r.model ?? ''),
    inputTokens: Number(r.input_tokens ?? 0),
    outputTokens: Number(r.output_tokens ?? 0),
    input: r.input_json ?? null,
    output: r.output_json ?? null,
    error: String(r.error ?? ''),
  }));
}
