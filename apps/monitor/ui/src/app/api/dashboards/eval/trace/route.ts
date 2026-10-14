/**
 * LLM Eval case execution trace API endpoint.
 *
 * @module
 */
import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { queryEvalCaseTrace } from '@/lib/eval-queries';

/**
 * Handles GET requests to retrieve detailed span events (tool and LLM calls) for a specific eval case.
 *
 * @param req - Incoming HTTP request with `runId` and `caseIndex` query parameters.
 * @returns JSON list of span trace events.
 */
export async function GET(req: NextRequest) {
  const denied = await requireAdmin(req);
  if (denied) return denied;

  const runId = req.nextUrl.searchParams.get('runId') ?? '';
  const caseIndexRaw = req.nextUrl.searchParams.get('caseIndex') ?? '';
  const caseIndex = Number(caseIndexRaw);
  if (!runId || !Number.isInteger(caseIndex) || caseIndex < 0) {
    return NextResponse.json({ events: [], error: 'runId and caseIndex are required' }, { status: 400 });
  }

  try {
    const events = await queryEvalCaseTrace(getPool(), runId, caseIndex);
    return NextResponse.json({ events });
  } catch (error) {
    console.error('dashboard eval trace failed:', error);
    return NextResponse.json({ events: [], error: 'query failed' }, { status: 500 });
  }
}
