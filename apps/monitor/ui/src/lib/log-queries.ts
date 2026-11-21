/**
 * PostgreSQL search and pagination queries for job execution logs.
 *
 * @module
 */
import type { Pool } from 'pg';
import type { JobLog } from '../types';

/**
 * Raw log record structure retrieved from database view.
 * @property id Optional database log identifier.
 * @property job_id Job identifier associated with the log.
 * @property workspace_id Workspace identifier associated with the log.
 * @property document_id Document identifier associated with the log.
 * @property level Log severity level.
 * @property event Structured event name.
 * @property message Human-readable log message.
 * @property detail_json Structured detail payload.
 * @property created_at Log creation timestamp.
 */
export interface RawLogRow {
  id?: string;
  job_id: string;
  workspace_id: string;
  document_id: string;
  level: string;
  event: string;
  message: string;
  detail_json: unknown;
  created_at: Date | string;
}

/**
 * Filter criteria for searching and paginating log entries.
 * @property query Free-text search across event, message, and detail JSON.
 * @property workspaceId Optional workspace scope.
 * @property documentId Optional document scope.
 * @property jobId Optional job scope; takes precedence over broader scopes.
 * @property levels Severity levels to include.
 * @property events Structured event names to include.
 * @property fromTimestamp Inclusive lower timestamp bound.
 * @property toTimestamp Inclusive upper timestamp bound.
 * @property pageToken Cursor for the next page.
 * @property limit Requested page size, clamped by the query implementation.
 */
export interface SearchFilters {
  query: string;
  workspaceId: string;
  documentId: string;
  jobId: string;
  levels: string[];
  events: string[];
  fromTimestamp: string;
  toTimestamp: string;
  pageToken: string;
  limit: number;
}

/**
 * Paginated log search query result.
 * @property logs Log rows matching the supplied filters.
 * @property nextPageToken Cursor to pass to the next search, or an empty string when exhausted.
 */
export interface SearchResult {
  logs: RawLogRow[];
  nextPageToken: string;
}

/**
 * Searches job logs based on entity scopes, full-text filters, severity levels, and time bounds.
 *
 * @param pool PostgreSQL client pool.
 * @param f Search filter parameters.
 * @returns Promise resolving to matched logs and next page token.
 */
export async function searchLogs(pool: Pool, f: SearchFilters): Promise<SearchResult> {
  const limit = Math.min(Math.max(f.limit || 500, 1), 1000);
  const args: unknown[] = [];
  const conds: string[] = [];

  if (f.jobId) {
    args.push(f.jobId);
    conds.push(`job_id = $${args.length}`);
  } else if (f.documentId) {
    args.push(f.documentId);
    conds.push(`document_id = $${args.length}`);
  } else if (f.workspaceId) {
    args.push(f.workspaceId);
    conds.push(`workspace_id = $${args.length}`);
  }

  if (f.levels.length > 0) {
    args.push(f.levels);
    conds.push(`level = ANY($${args.length}::text[])`);
  }
  if (f.events.length > 0) {
    args.push(f.events);
    conds.push(`event = ANY($${args.length}::text[])`);
  }
  if (f.query) {
    args.push(`%${f.query}%`);
    const idx = args.length;
    conds.push(`(event ILIKE $${idx} OR message ILIKE $${idx} OR detail_json::text ILIKE $${idx})`);
  }
  if (f.fromTimestamp) {
    args.push(f.fromTimestamp);
    conds.push(`created_at >= $${args.length}`);
  }
  if (f.toTimestamp) {
    args.push(f.toTimestamp);
    conds.push(`created_at <= $${args.length}`);
  }
  if (f.pageToken) {
    args.push(f.pageToken);
    conds.push(`created_at < $${args.length}`);
  }

  const where = conds.length > 0 ? `WHERE ${conds.join(' AND ')}` : '';
  args.push(limit + 1);
  const sql = `
    SELECT id, job_id, workspace_id, document_id, level, event, message, detail_json, created_at
    FROM v_job_logs
    ${where}
    ORDER BY created_at DESC
    LIMIT $${args.length}
  `;
  const { rows } = await pool.query(sql, args);
  const hasMore = rows.length > limit;
  const logs = (hasMore ? rows.slice(0, limit) : rows) as RawLogRow[];
  const nextPageToken = hasMore
    ? new Date((logs[logs.length - 1] as RawLogRow).created_at).toISOString()
    : '';
  return { logs, nextPageToken };
}

/**
 * Converts a database RawLogRow into the OpenAPI-compatible JobLog wire model.
 *
 * @param r Raw database log row.
 * @returns Serialized JobLog object.
 */
export function logRowToWireFormat(r: RawLogRow): JobLog {
  return {
    timestamp:
      r.created_at instanceof Date
        ? r.created_at.toISOString()
        : String(r.created_at ?? ''),
    level: r.level,
    event: r.event,
    message: r.message,
    detailJson: r.detail_json ? JSON.stringify(r.detail_json) : '',
    source: '',
    sourceId: '',
    jobId: r.job_id,
    documentId: r.document_id,
    workspaceId: r.workspace_id,
  };
}
