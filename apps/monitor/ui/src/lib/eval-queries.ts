import type { Pool } from 'pg';
import type { Period } from './dashboard-queries';

function periodInterval(period: Period): string {
  switch (period) {
    case 'today': return "date_trunc('day', NOW())";
    case '7d': return "NOW() - INTERVAL '7 days'";
    case '30d': return "NOW() - INTERVAL '30 days'";
  }
}

export interface EvalTrendPoint {
  date: string;
  runs: number;
  cases: number;
  passed: number;
  passRate: number;
}

export interface EvalPromptSourceRow {
  promptSource: string;
  runs: number;
  cases: number;
  passRate: number;
  avgDurationMs: number;
}

export interface EvalModelRow {
  model: string;
  cases: number;
  passRate: number;
  avgDurationMs: number;
  p95DurationMs: number;
  inputTokens: number;
  outputTokens: number;
}

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
}

export interface EvalCaseRow {
  runId: string;
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

export interface EvalData {
  totalRuns: number;
  totalCases: number;
  passRate: number;
  avgRunDurationMs: number;
  p95CaseDurationMs: number;
  inputTokens: number;
  outputTokens: number;
  trend: EvalTrendPoint[];
  byPromptSource: EvalPromptSourceRow[];
  byModel: EvalModelRow[];
  recentRuns: EvalRunRow[];
  recentCases: EvalCaseRow[];
  recentFailures: EvalCaseRow[];
  slowestCases: EvalCaseRow[];
}

export async function queryEval(pool: Pool, period: Period): Promise<EvalData> {
  const since = periodInterval(period);
  const dateTrunc = period === 'today' ? 'hour' : 'day';
  const dateLabel = period === 'today'
    ? `to_char(date_trunc('hour', started_at), 'HH24:00')`
    : `date_trunc('day', started_at)::date::text`;

  const [overview, latency, trend, byPromptSource, byModel, recentRuns, recentCases, recentFailures, slowestCases] = await Promise.all([
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
      SELECT COALESCE(
        PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms),
        0
      ) AS p95_case_duration_ms
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
        COALESCE(ROUND(AVG(duration_ms)), 0) AS avg_duration_ms
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
        started_at
      FROM v_eval_runs
      WHERE started_at >= ${since}
      ORDER BY started_at DESC
      LIMIT 25
    `),
    pool.query(`
      SELECT
        run_id,
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
  const totalCases = Number(ov.total_cases ?? 0);
  const passedCases = Number(ov.passed_cases ?? 0);
  const toISO = (value: unknown) => value instanceof Date ? value.toISOString() : String(value ?? '');
  const caseRow = (r: Record<string, unknown>): EvalCaseRow => ({
    runId: String(r.run_id),
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
    p95CaseDurationMs: Number(latency.rows[0]?.p95_case_duration_ms ?? 0),
    inputTokens: Number(ov.input_tokens ?? 0),
    outputTokens: Number(ov.output_tokens ?? 0),
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
    })),
    recentCases: recentCases.rows.map(caseRow),
    recentFailures: recentFailures.rows.map(caseRow),
    slowestCases: slowestCases.rows.map(caseRow),
  };
}
