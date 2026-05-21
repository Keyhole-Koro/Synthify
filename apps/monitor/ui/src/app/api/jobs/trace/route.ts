import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { queryJobTrace } from '@/lib/trace-queries';

export async function GET(req: NextRequest) {
  const jobId = req.nextUrl.searchParams.get('jobId');
  if (!jobId) {
    return NextResponse.json({ error: 'jobId required' }, { status: 400 });
  }
  try {
    const data = await queryJobTrace(getPool(), jobId);
    return NextResponse.json(data);
  } catch (error) {
    console.error('trace query failed:', error);
    return NextResponse.json({ jobId, toolCalls: [] }, { status: 500 });
  }
}
