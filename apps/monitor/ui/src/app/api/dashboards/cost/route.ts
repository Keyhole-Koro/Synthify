import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { queryCost, type Period } from '@/lib/dashboard-queries';

const EMPTY_COST = {
  totalCostMinor: 0,
  creditCostMinor: 0,
  stripeCostMinor: 0,
  dailyTrend: [],
  byModel: [],
  topJobs: [],
};

function parsePeriod(value: string | null): Period {
  return value === 'today' || value === '7d' || value === '30d' ? value : '7d';
}

export async function GET(req: NextRequest) {
  const period = parsePeriod(req.nextUrl.searchParams.get('period'));
  try {
    const data = await queryCost(getPool(), period);
    return NextResponse.json(data);
  } catch (error) {
    console.error('dashboard cost failed:', error);
    return NextResponse.json({ ...EMPTY_COST, error: 'query failed' }, { status: 500 });
  }
}
