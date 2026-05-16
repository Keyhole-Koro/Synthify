import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { queryWorkspace, type Period } from '@/lib/dashboard-queries';

const EMPTY_WORKSPACE = {
  totalWorkspaces: 0,
  dormantWorkspaces: 0,
  docsByDay: [],
  itemsByDay: [],
};

function parsePeriod(value: string | null): Period {
  return value === 'today' || value === '7d' || value === '30d' ? value : '7d';
}

export async function GET(req: NextRequest) {
  const period = parsePeriod(req.nextUrl.searchParams.get('period'));
  try {
    const data = await queryWorkspace(getPool(), period);
    return NextResponse.json(data);
  } catch (error) {
    console.error('dashboard workspace failed:', error);
    return NextResponse.json({ ...EMPTY_WORKSPACE, error: 'query failed' }, { status: 500 });
  }
}
