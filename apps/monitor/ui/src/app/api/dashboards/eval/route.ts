/**
 * LLM Evaluation aggregated metrics API endpoint.
 *
 * @module
 */
import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { queryEval } from '@/lib/eval-queries';
import type { Period } from '@/lib/dashboard-queries';

/** Empty fallback evaluation metrics payload. */
const EMPTY_EVAL = {
  totalRuns: 0,
  totalCases: 0,
  passRate: 0,
  avgRunDurationMs: 0,
  p95CaseDurationMs: 0,
  inputTokens: 0,
  outputTokens: 0,
  funnel: {
    totalCases: 0,
    executionCompleted: 0,
    schemaValid: 0,
    passed: 0,
    executionErrors: 0,
    schemaInvalid: 0,
    assertionFailures: 0,
  },
  trend: [],
  byPromptSource: [],
  byModel: [],
  recentRuns: [],
  recentCases: [],
  recentFailures: [],
  slowestCases: [],
};

/**
 * Validates and normalizes the period query parameter into a recognized Period enum value.
 *
 * @param value - Raw URL search parameter value.
 * @returns Validated Period string ('today', '7d', or '30d').
 */
function parsePeriod(value: string | null): Period {
  return value === 'today' || value === '7d' || value === '30d' ? value : '7d';
}

/**
 * Handles GET requests to retrieve aggregated LLM evaluation analytics across prompt sources,
 * models, funnel stages, and historical runs.
 *
 * @param req - Incoming HTTP request with admin authorization and period filter.
 * @returns Evaluation metrics JSON payload.
 */
export async function GET(req: NextRequest) {
  const denied = await requireAdmin(req);
  if (denied) return denied;

  const period = parsePeriod(req.nextUrl.searchParams.get('period'));
  try {
    return NextResponse.json(await queryEval(getPool(), period));
  } catch (error) {
    console.error('dashboard eval failed:', error);
    return NextResponse.json({ ...EMPTY_EVAL, error: 'query failed' }, { status: 500 });
  }
}
