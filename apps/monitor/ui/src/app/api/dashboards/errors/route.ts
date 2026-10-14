/**
 * Error statistics and alert metrics API endpoint.
 *
 * @module
 */
import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { queryErrors, type Period } from '@/lib/dashboard-queries';

/** Empty fallback error metrics structure. */
const EMPTY_ERRORS = {
  totalErrors: 0,
  recentBurst: false,
  byDay: [],
  topEvents: [],
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
 * Handles GET requests to retrieve error counts, burst status, and error event rankings.
 *
 * @param req - Incoming HTTP request with admin authorization and period filter.
 * @returns Error metrics JSON payload.
 */
export async function GET(req: NextRequest) {
  const denied = await requireAdmin(req);
  if (denied) return denied;

  const period = parsePeriod(req.nextUrl.searchParams.get('period'));
  try {
    const data = await queryErrors(getPool(), period);
    return NextResponse.json(data);
  } catch (error) {
    console.error('dashboard errors failed:', error);
    return NextResponse.json({ ...EMPTY_ERRORS, error: 'query failed' }, { status: 500 });
  }
}
