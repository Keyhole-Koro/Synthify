/**
 * Individual job logs retrieval API endpoint with cursor-based pagination.
 *
 * @module
 */
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import type { JobLog, ListJobLogsResponse } from '@/types';

/**
 * Handles GET requests to retrieve paginated log entries for a single background job.
 *
 * @param req - Incoming HTTP request with optional `page_token` and `limit` query parameters.
 * @param context - Route context containing the `jobId` parameter promise.
 * @returns ListJobLogsResponse JSON payload containing logs and next page token.
 */
export async function GET(
  req: Request,
  { params }: { params: { jobId: string } },
) {
  const denied = await requireAdmin(req);
  if (denied) return denied;

  const { jobId } = params;
  if (!jobId) {
    return Response.json({ error: 'job_id is required' }, { status: 400 });
  }
  const url = new URL(req.url);
  const pageToken = url.searchParams.get('page_token') ?? '';
  const limitParam = Number.parseInt(url.searchParams.get('limit') ?? '500', 10);
  const limit = Number.isFinite(limitParam) && limitParam > 0 ? Math.min(limitParam, 1000) : 500;

  try {
    const pool = getPool();
    // page_token is a timestamp cursor on created_at (ISO)
    const args: unknown[] = [jobId];
    let where = 'WHERE job_id = $1';
    if (pageToken) {
      args.push(pageToken);
      where += ` AND created_at < $${args.length}`;
    }
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
    const logs = (hasMore ? rows.slice(0, limit) : rows).map(rowToLog);
    const nextPageToken = hasMore
      ? new Date(rows[limit - 1].created_at).toISOString()
      : '';
    const response: ListJobLogsResponse = { logs, nextPageToken };
    return Response.json(response);
  } catch (error) {
    console.error('monitor listJobLogs failed:', error);
    const response: ListJobLogsResponse = { logs: [], nextPageToken: '' };
    return Response.json(response, { status: 500 });
  }
}

/**
 * Converts a database row from `v_job_logs` to the frontend `JobLog` shape.
 *
 * @param r - Database result row.
 * @returns Formatted JobLog object.
 */
function rowToLog(r: Record<string, unknown>): JobLog {
  return {
    timestamp:
      r.created_at instanceof Date
        ? r.created_at.toISOString()
        : String(r.created_at ?? ''),
    level: String(r.level ?? ''),
    event: String(r.event ?? ''),
    message: String(r.message ?? ''),
    detailJson: r.detail_json ? JSON.stringify(r.detail_json) : '',
    source: '',
    sourceId: '',
    jobId: String(r.job_id ?? ''),
    documentId: String(r.document_id ?? ''),
    workspaceId: String(r.workspace_id ?? ''),
  };
}
