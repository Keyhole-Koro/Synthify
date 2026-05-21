import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { queryJobHealth, type Period } from '@/lib/dashboard-queries';

const EMPTY_JOB_HEALTH = {
  totalJobs: 0,
  successRate: 0,
  avgProcessingMs: 0,
  p95ProcessingMs: 0,
  byDay: [],
  stageFailures: [],
  topRetryJobs: [],
};

function parsePeriod(value: string | null): Period {
  return value === 'today' || value === '7d' || value === '30d' ? value : '7d';
}

export async function GET(req: NextRequest) {
  const period = parsePeriod(req.nextUrl.searchParams.get('period'));
  try {
    const data = await queryJobHealth(getPool(), period);
    return NextResponse.json(data);
  } catch (error) {
    console.error('dashboard job-health failed:', error);
    return NextResponse.json({ ...EMPTY_JOB_HEALTH, error: 'query failed' }, { status: 500 });
  }
}
