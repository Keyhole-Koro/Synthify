/**
 * Job logs search and filtering API endpoint.
 *
 * @module
 */
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { logRowToWireFormat, searchLogs } from '@/lib/log-queries';
import type { ListJobLogsResponse, SearchJobLogsRequest } from '@/types';

/**
 * Handles POST requests to search and filter job logs by keyword, log levels, events, and timestamps.
 *
 * @param req - Incoming HTTP request with SearchJobLogsRequest payload.
 * @returns ListJobLogsResponse JSON payload containing matching logs and pagination cursor.
 */
export async function POST(req: Request) {
  const denied = await requireAdmin(req);
  if (denied) return denied;

  let body: SearchJobLogsRequest;
  try {
    body = (await req.json()) as SearchJobLogsRequest;
  } catch {
    return Response.json({ error: 'invalid JSON body' }, { status: 400 });
  }

  try {
    const pool = getPool();
    const { logs, nextPageToken } = await searchLogs(pool, {
      query: body.query ?? '',
      workspaceId: body.workspaceId ?? '',
      documentId: body.documentId ?? '',
      jobId: body.jobId ?? '',
      levels: body.levels ?? [],
      events: body.events ?? [],
      fromTimestamp: body.fromTimestamp ?? '',
      toTimestamp: body.toTimestamp ?? '',
      pageToken: body.pageToken ?? '',
      limit: body.limit ?? 500,
    });
    const response: ListJobLogsResponse = {
      logs: logs.map(logRowToWireFormat),
      nextPageToken,
    };
    return Response.json(response);
  } catch (error) {
    console.error('monitor searchJobLogs failed:', error);
    const response: ListJobLogsResponse = { logs: [], nextPageToken: '' };
    return Response.json(response, { status: 500 });
  }
}
