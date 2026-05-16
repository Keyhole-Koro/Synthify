import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { logRowToWireFormat, searchLogs } from '@/lib/log-queries';
import type { ListJobLogsResponse, SearchJobLogsRequest } from '@/types';

export async function POST(req: NextRequest) {
  let body: SearchJobLogsRequest;
  try {
    body = (await req.json()) as SearchJobLogsRequest;
  } catch {
    return NextResponse.json({ error: 'invalid JSON body' }, { status: 400 });
  }

  try {
    const pool = getPool();
    const { logs, nextPageToken } = await searchLogs(pool, {
      query: body.query ?? '',
      workspaceId: body.workspaceId ?? '',
      documentId: body.documentId ?? '',
      jobId: body.jobId ?? '',
      levels: body.levels ?? [],
      events: body.events ?? [],
      fromTimestamp: body.fromTimestamp ?? '',
      toTimestamp: body.toTimestamp ?? '',
      pageToken: body.pageToken ?? '',
      limit: body.limit ?? 500,
    });
    const response: ListJobLogsResponse = {
      logs: logs.map(logRowToWireFormat),
      nextPageToken,
    };
    return NextResponse.json(response);
  } catch (error) {
    console.error('log-viewer searchJobLogs failed:', error);
    const response: ListJobLogsResponse = { logs: [], nextPageToken: '' };
    return NextResponse.json(response, { status: 500 });
  }
}
