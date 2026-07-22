import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { queryEval } from '@/lib/eval-queries';
import type { Period } from '@/lib/dashboard-queries';

const EMPTY_EVAL = {
  totalRuns: 0,
  totalCases: 0,
  passRate: 0,
  avgRunDurationMs: 0,
  p95CaseDurationMs: 0,
  inputTokens: 0,
  outputTokens: 0,
  trend: [],
  byPromptSource: [],
  byModel: [],
  recentRuns: [],
  recentCases: [],
  recentFailures: [],
  slowestCases: [],
};

function parsePeriod(value: string | null): Period {
  return value === 'today' || value === '7d' || value === '30d' ? value : '7d';
}

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
