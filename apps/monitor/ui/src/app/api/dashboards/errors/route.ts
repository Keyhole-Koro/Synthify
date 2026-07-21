import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { requireAdmin } from '@/lib/auth-server';
import { queryErrors, type Period } from '@/lib/dashboard-queries';

const EMPTY_ERRORS = {
  totalErrors: 0,
  recentBurst: false,
  byDay: [],
  topEvents: [],
};

function parsePeriod(value: string | null): Period {
  return value === 'today' || value === '7d' || value === '30d' ? value : '7d';
}

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
