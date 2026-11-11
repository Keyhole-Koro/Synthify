/**
 * Job trace tool execution timeline API endpoint.
 *
 * @module
 */
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { queryJobTrace } from '@/lib/trace-queries';

/**
 * Handles GET requests to retrieve the chronological tool call mutations executed by a specific job.
 *
 * @param req - Incoming HTTP request with `jobId` search parameter.
 * @returns JobTraceData JSON payload containing list of tool calls.
 */
export async function GET(req: Request) {
  const denied = await requireAdmin(req);
  if (denied) return denied;

  const jobId = new URL(req.url).searchParams.get('jobId');
  if (!jobId) {
    return Response.json({ error: 'jobId required' }, { status: 400 });
  }
  try {
    const data = await queryJobTrace(getPool(), jobId);
    return Response.json(data);
  } catch (error) {
    console.error('trace query failed:', error);
    return Response.json({ jobId, toolCalls: [] }, { status: 500 });
  }
}
