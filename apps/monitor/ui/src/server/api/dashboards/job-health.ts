/**
 * Job health and pipeline processing statistics API endpoint.
 *
 * @module
 */
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { queryJobHealth, type Period } from '@/lib/dashboard-queries';

/** Empty fallback job health statistics payload. */
const EMPTY_JOB_HEALTH = {
  totalJobs: 0,
  successRate: 0,
  avgProcessingMs: 0,
  p50ProcessingMs: 0,
  p95ProcessingMs: 0,
  byDay: [],
  stageFailures: [],
  topRetryJobs: [],
  errorMessages: [],
  byMimeType: [],
  activeFailures: [],
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
 * Handles GET requests to retrieve overall job processing throughput, latencies, and stage failure distributions.
 *
 * @param req - Incoming HTTP request with admin authorization and period filter.
 * @returns Job health metrics JSON payload.
 */
export async function GET(req: Request) {
  const denied = await requireAdmin(req);
  if (denied) return denied;

  const period = parsePeriod(new URL(req.url).searchParams.get('period'));
  try {
    const data = await queryJobHealth(getPool(), period);
    return Response.json(data);
  } catch (error) {
    console.error('dashboard job-health failed:', error);
    return Response.json({ ...EMPTY_JOB_HEALTH, error: 'query failed' }, { status: 500 });
  }
}
