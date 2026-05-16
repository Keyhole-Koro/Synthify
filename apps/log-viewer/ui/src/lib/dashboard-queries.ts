import type { Pool } from 'pg';

export type Period = 'today' | '7d' | '30d';

function periodInterval(period: Period): string {
  switch (period) {
    case 'today': return "date_trunc('day', NOW())";
    case '7d':    return "NOW() - INTERVAL '7 days'";
    case '30d':   return "NOW() - INTERVAL '30 days'";
  }
}

// ── Job Health ────────────────────────────────────────────────────────────────

export interface JobStatusByDay {
  date: string;
  succeeded: number;
  failed: number;
  running: number;
  queued: number;
}

export interface StageFailure {
  stage: string;
  count: number;
}

export interface TopRetryJob {
  jobId: string;
  documentId: string;
  retryCount: number;
}

export interface ErrorMessage {
  message: string;
  count: number;
}

export interface MimeTypeFailure {
  mimeType: string;
  failed: number;
  total: number;
}

export interface ActiveFailure {
  jobId: string;
  documentId: string;
  currentStage: string;
  errorMessage: string;
  updatedAt: string;
}

export interface JobHealthData {
  totalJobs: number;
  successRate: number;
  avgProcessingMs: number;
  p95ProcessingMs: number;
  byDay: JobStatusByDay[];
  stageFailures: StageFailure[];
  topRetryJobs: TopRetryJob[];
  errorMessages: ErrorMessage[];
  byMimeType: MimeTypeFailure[];
  activeFailures: ActiveFailure[];
}

export async function queryJobHealth(pool: Pool, period: Period): Promise<JobHealthData> {
  const since = periodInterval(period);

  // today は時間単位、それ以外は日単位
  const dateTrunc = period === 'today' ? 'hour' : 'day';
  const dateLabel = period === 'today'
    ? `to_char(date_trunc('hour', created_at), 'HH24:00')`
    : `date_trunc('day', created_at)::date::text`;

  const [overview, byDay, stageFailures, topRetry, errorMessages, byMimeType, activeFailures] = await Promise.all([
    pool.query(`
      SELECT
        COUNT(*)                                                         AS total,
        COUNT(*) FILTER (WHERE status = 'succeeded')                    AS succeeded,
        ROUND(
          AVG(EXTRACT(EPOCH FROM (updated_at - created_at)) * 1000)
        )                                                                AS avg_ms,
        PERCENTILE_CONT(0.95) WITHIN GROUP (
          ORDER BY EXTRACT(EPOCH FROM (updated_at - created_at)) * 1000
        )                                                                AS p95_ms
      FROM v_processing_jobs
      WHERE created_at >= ${since}
    `),
    pool.query(`
      SELECT
        ${dateLabel} AS date,
        COUNT(*) FILTER (WHERE status = 'succeeded') AS succeeded,
        COUNT(*) FILTER (WHERE status = 'failed')    AS failed,
        COUNT(*) FILTER (WHERE status = 'running')   AS running,
        COUNT(*) FILTER (WHERE status = 'queued')    AS queued
      FROM v_processing_jobs
      WHERE created_at >= ${since}
      GROUP BY date_trunc('${dateTrunc}', created_at)
      ORDER BY date_trunc('${dateTrunc}', created_at)
    `),
    pool.query(`
      SELECT current_stage AS stage, COUNT(*) AS count
      FROM v_processing_jobs
      WHERE status = 'failed' AND created_at >= ${since} AND current_stage <> ''
      GROUP BY 1
      ORDER BY 2 DESC
      LIMIT 10
    `),
    pool.query(`
      SELECT job_id, document_id, retry_count
      FROM v_processing_jobs
      WHERE retry_count > 0 AND created_at >= ${since}
      ORDER BY retry_count DESC
      LIMIT 10
    `),
    pool.query(`
      SELECT error_message AS message, COUNT(*) AS count
      FROM v_processing_jobs
      WHERE status = 'failed' AND error_message <> '' AND created_at >= ${since}
      GROUP BY error_message
      ORDER BY count DESC
      LIMIT 10
    `),
    pool.query(`
      SELECT
        d.mime_type,
        COUNT(*) FILTER (WHERE j.status = 'failed') AS failed,
        COUNT(*) AS total
      FROM v_processing_jobs j
      JOIN v_documents d ON d.document_id = j.document_id
      WHERE j.created_at >= ${since}
      GROUP BY d.mime_type
      ORDER BY failed DESC
      LIMIT 10
    `),
    pool.query(`
      SELECT job_id, document_id, current_stage, error_message, updated_at
      FROM v_processing_jobs
      WHERE status = 'failed' AND created_at >= ${since}
      ORDER BY updated_at DESC
      LIMIT 20
    `),
  ]);

  const ov = overview.rows[0];
  const total = Number(ov.total ?? 0);
  const succeeded = Number(ov.succeeded ?? 0);

  return {
    totalJobs: total,
    successRate: total > 0 ? succeeded / total : 0,
    avgProcessingMs: Number(ov.avg_ms ?? 0),
    p95ProcessingMs: Number(ov.p95_ms ?? 0),
    byDay: byDay.rows.map((r) => ({
      date: String(r.date),
      succeeded: Number(r.succeeded),
      failed: Number(r.failed),
      running: Number(r.running),
      queued: Number(r.queued),
    })),
    stageFailures: stageFailures.rows.map((r) => ({
      stage: String(r.stage),
      count: Number(r.count),
    })),
    topRetryJobs: topRetry.rows.map((r) => ({
      jobId: String(r.job_id),
      documentId: String(r.document_id),
      retryCount: Number(r.retry_count),
    })),
    errorMessages: errorMessages.rows.map((r) => ({
      message: String(r.message),
      count: Number(r.count),
    })),
    byMimeType: byMimeType.rows.map((r) => ({
      mimeType: String(r.mime_type),
      failed: Number(r.failed),
      total: Number(r.total),
    })),
    activeFailures: activeFailures.rows.map((r) => ({
      jobId: String(r.job_id),
      documentId: String(r.document_id),
      currentStage: String(r.current_stage),
      errorMessage: String(r.error_message),
      updatedAt: r.updated_at instanceof Date ? r.updated_at.toISOString() : String(r.updated_at),
    })),
  };
}

// ── Cost & Usage ──────────────────────────────────────────────────────────────

export interface DailyCost {
  date: string;
  costMinor: number;
}

export interface ModelCost {
  model: string;
  costMinor: number;
  inputTokens: number;
  outputTokens: number;
}

export interface TopJobCost {
  jobId: string;
  costMinor: number;
}

export interface CostData {
  totalCostMinor: number;
  creditCostMinor: number;
  stripeCostMinor: number;
  dailyTrend: DailyCost[];
  byModel: ModelCost[];
  topJobs: TopJobCost[];
}

export async function queryCost(pool: Pool, period: Period): Promise<CostData> {
  const since = periodInterval(period);

  const [overview, daily, byModel, topJobs] = await Promise.all([
    pool.query(`
      SELECT
        COALESCE(SUM(cost_minor), 0)          AS total,
        COALESCE(SUM(credit_amount_minor), 0) AS credit,
        COALESCE(SUM(stripe_amount_minor), 0) AS stripe
      FROM v_usage_events
      WHERE created_at >= ${since}
    `),
    pool.query(`
      SELECT
        date_trunc('day', created_at)::date::text AS date,
        SUM(cost_minor) AS cost_minor
      FROM v_usage_events
      WHERE created_at >= ${since}
      GROUP BY 1
      ORDER BY 1
    `),
    pool.query(`
      SELECT
        model,
        SUM(cost_minor)    AS cost_minor,
        SUM(input_tokens)  AS input_tokens,
        SUM(output_tokens) AS output_tokens
      FROM v_usage_events
      WHERE created_at >= ${since}
      GROUP BY model
      ORDER BY cost_minor DESC
    `),
    pool.query(`
      SELECT job_id, SUM(cost_minor) AS cost_minor
      FROM v_usage_events
      WHERE created_at >= ${since} AND job_id <> ''
      GROUP BY job_id
      ORDER BY cost_minor DESC
      LIMIT 10
    `),
  ]);

  const ov = overview.rows[0];
  return {
    totalCostMinor: Number(ov.total ?? 0),
    creditCostMinor: Number(ov.credit ?? 0),
    stripeCostMinor: Number(ov.stripe ?? 0),
    dailyTrend: daily.rows.map((r) => ({
      date: String(r.date),
      costMinor: Number(r.cost_minor),
    })),
    byModel: byModel.rows.map((r) => ({
      model: String(r.model),
      costMinor: Number(r.cost_minor),
      inputTokens: Number(r.input_tokens),
      outputTokens: Number(r.output_tokens),
    })),
    topJobs: topJobs.rows.map((r) => ({
      jobId: String(r.job_id),
      costMinor: Number(r.cost_minor),
    })),
  };
}

// ── Workspace Activity ────────────────────────────────────────────────────────

export interface DocsByDay {
  date: string;
  count: number;
}

export interface ItemsByDay {
  date: string;
  count: number;
}

export interface WorkspaceData {
  totalWorkspaces: number;
  dormantWorkspaces: number;
  docsByDay: DocsByDay[];
  itemsByDay: ItemsByDay[];
}

export async function queryWorkspace(pool: Pool, period: Period): Promise<WorkspaceData> {
  const since = periodInterval(period);

  const [wsOverview, docsByDay, itemsByDay] = await Promise.all([
    pool.query(`
      SELECT
        COUNT(*)                                                                        AS total,
        COUNT(*) FILTER (WHERE updated_at < NOW() - INTERVAL '30 days')                AS dormant
      FROM v_workspaces
    `),
    pool.query(`
      SELECT
        date_trunc('day', created_at)::date::text AS date,
        COUNT(*) AS count
      FROM v_documents
      WHERE created_at >= ${since}
      GROUP BY 1
      ORDER BY 1
    `),
    pool.query(`
      SELECT
        date_trunc('day', created_at)::date::text AS date,
        COUNT(*) AS count
      FROM v_tree_items
      WHERE created_at >= ${since}
      GROUP BY 1
      ORDER BY 1
    `),
  ]);

  const ov = wsOverview.rows[0];
  return {
    totalWorkspaces: Number(ov.total ?? 0),
    dormantWorkspaces: Number(ov.dormant ?? 0),
    docsByDay: docsByDay.rows.map((r) => ({ date: String(r.date), count: Number(r.count) })),
    itemsByDay: itemsByDay.rows.map((r) => ({ date: String(r.date), count: Number(r.count) })),
  };
}

// ── Errors & Alerts ───────────────────────────────────────────────────────────

export interface ErrorsByDay {
  date: string;
  count: number;
}

export interface TopErrorEvent {
  event: string;
  count: number;
}

export interface ErrorsData {
  totalErrors: number;
  recentBurst: boolean;
  byDay: ErrorsByDay[];
  topEvents: TopErrorEvent[];
}

export async function queryErrors(pool: Pool, period: Period): Promise<ErrorsData> {
  const since = periodInterval(period);

  const [overview, byDay, topEvents, burstCheck] = await Promise.all([
    pool.query(`
      SELECT COUNT(*) AS total
      FROM v_job_logs
      WHERE level = 'ERROR' AND created_at >= ${since}
    `),
    pool.query(`
      SELECT
        date_trunc('day', created_at)::date::text AS date,
        COUNT(*) AS count
      FROM v_job_logs
      WHERE level = 'ERROR' AND created_at >= ${since}
      GROUP BY 1
      ORDER BY 1
    `),
    pool.query(`
      SELECT event, COUNT(*) AS count
      FROM v_job_logs
      WHERE level = 'ERROR' AND created_at >= ${since}
      GROUP BY event
      ORDER BY count DESC
      LIMIT 10
    `),
    pool.query(`
      SELECT COUNT(*) AS recent
      FROM v_job_logs
      WHERE level = 'ERROR' AND created_at >= NOW() - INTERVAL '5 minutes'
    `),
  ]);

  const recentCount = Number(burstCheck.rows[0]?.recent ?? 0);
  return {
    totalErrors: Number(overview.rows[0]?.total ?? 0),
    recentBurst: recentCount >= 10,
    byDay: byDay.rows.map((r) => ({ date: String(r.date), count: Number(r.count) })),
    topEvents: topEvents.rows.map((r) => ({ event: String(r.event), count: Number(r.count) })),
  };
}
