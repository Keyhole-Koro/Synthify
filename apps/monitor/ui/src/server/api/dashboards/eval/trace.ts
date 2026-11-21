/**
 * LLM Eval case execution trace API endpoint.
 *
 * @module
 */
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { queryEvalCaseTrace } from '@/lib/eval-queries';

/**
 * Handles GET requests to retrieve detailed span events (tool and LLM calls) for a specific eval case.
 *
 * @param req - Incoming HTTP request with `runId` and `caseIndex` query parameters.
 * @returns JSON list of span trace events.
 */
export async function GET(req: Request) {
  const denied = await requireAdmin(req);
  if (denied) return denied;

  const runId = new URL(req.url).searchParams.get('runId') ?? '';
  const caseIndexRaw = new URL(req.url).searchParams.get('caseIndex') ?? '';
  const caseIndex = Number(caseIndexRaw);
  if (!runId || !Number.isInteger(caseIndex) || caseIndex < 0) {
    return Response.json({ events: [], error: 'runId and caseIndex are required' }, { status: 400 });
  }

  try {
    const events = await queryEvalCaseTrace(getPool(), runId, caseIndex);
    return Response.json({ events });
  } catch (error) {
    console.error('dashboard eval trace failed:', error);
    return Response.json({ events: [], error: 'query failed' }, { status: 500 });
  }
}
