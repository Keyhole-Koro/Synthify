import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import type { JobLog, ListJobLogsResponse } from '@/types';

// GET /api/jobs/[jobId]/logs?page_token=&limit=500
export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ jobId: string }> },
) {
  const { jobId } = await params;
  if (!jobId) {
    return NextResponse.json({ error: 'job_id is required' }, { status: 400 });
  }
  const url = new URL(req.url);
  const pageToken = url.searchParams.get('page_token') ?? '';
  const limitParam = Number.parseInt(url.searchParams.get('limit') ?? '500', 10);
  const limit = Number.isFinite(limitParam) && limitParam > 0 ? Math.min(limitParam, 1000) : 500;

  try {
    const pool = getPool();
    // page_token は created_at (ISO) を載せる単純な cursor
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
    return NextResponse.json(response);
  } catch (error) {
    console.error('log-viewer listJobLogs failed:', error);
    const response: ListJobLogsResponse = { logs: [], nextPageToken: '' };
    return NextResponse.json(response, { status: 500 });
  }
}

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
