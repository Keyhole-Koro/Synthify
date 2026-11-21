/**
 * Workspace activity and resource creation statistics API endpoint.
 *
 * @module
 */
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { queryWorkspace, type Period } from '@/lib/dashboard-queries';

/** Empty fallback workspace activity payload. */
const EMPTY_WORKSPACE = {
  totalWorkspaces: 0,
  dormantWorkspaces: 0,
  docsByDay: [],
  itemsByDay: [],
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
 * Handles GET requests to retrieve workspace counts, dormant rates, and creation throughput.
 *
 * @param req - Incoming HTTP request with admin authorization and period filter.
 * @returns Workspace activity JSON payload.
 */
export async function GET(req: Request) {
  const denied = await requireAdmin(req);
  if (denied) return denied;

  const period = parsePeriod(new URL(req.url).searchParams.get('period'));
  try {
    const data = await queryWorkspace(getPool(), period);
    return Response.json(data);
  } catch (error) {
    console.error('dashboard workspace failed:', error);
    return Response.json({ ...EMPTY_WORKSPACE, error: 'query failed' }, { status: 500 });
  }
}
