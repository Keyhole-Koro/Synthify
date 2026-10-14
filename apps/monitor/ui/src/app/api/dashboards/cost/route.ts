/**
 * Cost and usage dashboard metrics API endpoint.
 *
 * @module
 */
import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { queryCost, type Period } from '@/lib/dashboard-queries';

/** Empty fallback cost payload returned on query errors. */
const EMPTY_COST = {
  totalCostMinor: 0,
  creditCostMinor: 0,
  stripeCostMinor: 0,
  dailyTrend: [],
  byModel: [],
  topJobs: [],
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
 * Handles GET requests to retrieve AI processing cost metrics and model breakdowns.
 *
 * @param req - Incoming HTTP request with admin authorization and period filter.
 * @returns Cost metrics JSON payload.
 */
export async function GET(req: NextRequest) {
  const denied = await requireAdmin(req);
  if (denied) return denied;

  const period = parsePeriod(req.nextUrl.searchParams.get('period'));
  try {
    const data = await queryCost(getPool(), period);
    return NextResponse.json(data);
  } catch (error) {
    console.error('dashboard cost failed:', error);
    return NextResponse.json({ ...EMPTY_COST, error: 'query failed' }, { status: 500 });
  }
}
