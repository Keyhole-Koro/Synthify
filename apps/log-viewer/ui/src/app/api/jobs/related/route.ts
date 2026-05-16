import { NextRequest, NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import { logRowToWireFormat } from '@/lib/log-queries';
import type { JobLog, ListRelatedJobLogsRequest, ListRelatedJobLogsResponse } from '@/types';

// POST /api/jobs/related — scope に応じて関連 job を取って logs を group 化する。
export async function POST(req: NextRequest) {
  let body: ListRelatedJobLogsRequest;
  try {
    body = (await req.json()) as ListRelatedJobLogsRequest;
  } catch {
    return NextResponse.json({ error: 'invalid JSON body' }, { status: 400 });
  }
  const limit = Math.min(Math.max(body.limit || 100, 1), 500);

  try {
    const pool = getPool();
    // job 一覧の取得
    let jobsSQL: string;
    const jobsArgs: unknown[] = [];
    switch (body.scope) {
      case 'job': {
        if (!body.jobId) return NextResponse.json(emptyRelatedResponse());
        jobsArgs.push(body.jobId);
        jobsSQL = `
          SELECT job_id, document_id, workspace_id, status, created_at
          FROM v_processing_jobs WHERE job_id = $1
        `;
        break;
      }
      case 'document': {
        if (!body.documentId) return NextResponse.json(emptyRelatedResponse());
        jobsArgs.push(body.documentId);
        jobsSQL = `
          SELECT job_id, document_id, workspace_id, status, created_at
          FROM v_processing_jobs WHERE document_id = $1
          ORDER BY created_at DESC LIMIT 100
        `;
        break;
      }
      case 'workspace': {
        if (!body.workspaceId) return NextResponse.json(emptyRelatedResponse());
        jobsArgs.push(body.workspaceId);
        jobsSQL = `
          SELECT job_id, document_id, workspace_id, status, created_at
          FROM v_processing_jobs WHERE workspace_id = $1
          ORDER BY created_at DESC LIMIT 100
        `;
        break;
      }
      default:
        return NextResponse.json({ error: 'invalid scope' }, { status: 400 });
    }

    const { rows: jobRows } = await pool.query(jobsSQL, jobsArgs);
    if (jobRows.length === 0) {
      return NextResponse.json(emptyRelatedResponse());
    }

    const jobIds = jobRows.map((j) => j.job_id);
    const { rows: logRows } = await pool.query(
      `
        SELECT id, job_id, workspace_id, document_id, level, event, message, detail_json, created_at
        FROM v_job_logs
        WHERE job_id = ANY($1::text[])
        ORDER BY job_id, created_at ASC
        LIMIT $2
      `,
      [jobIds, limit * jobIds.length],
    );

    // group: workspace_id + document_id, jobs: その下のジョブ群
    const groupMap = new Map<string, {
      workspaceId: string;
      documentId: string;
      jobs: Array<{ jobId: string; status: number; createdAt: string; logs: JobLog[] }>;
    }>();
    const jobMap = new Map<string, { jobId: string; status: number; createdAt: string; logs: JobLog[] }>();

    for (const j of jobRows) {
      const key = `${j.workspace_id}::${j.document_id}`;
      if (!groupMap.has(key)) {
        groupMap.set(key, {
          workspaceId: j.workspace_id,
          documentId: j.document_id,
          jobs: [],
        });
      }
      const createdAt =
        j.created_at instanceof Date ? j.created_at.toISOString() : String(j.created_at ?? '');
      const job = { jobId: j.job_id, status: 0, createdAt, logs: [] as JobLog[] };
      groupMap.get(key)!.jobs.push(job);
      jobMap.set(j.job_id, job);
    }
    for (const r of logRows) {
      const j = jobMap.get(r.job_id);
      if (j) {
        j.logs.push(logRowToWireFormat(r));
      }
    }

    const response: ListRelatedJobLogsResponse = {
      groups: Array.from(groupMap.values()),
      nextPageToken: '',
    };
    return NextResponse.json(response);
  } catch (error) {
    console.error('log-viewer listRelatedJobLogs failed:', error);
    return NextResponse.json(emptyRelatedResponse(), { status: 500 });
  }
}

function emptyRelatedResponse(): ListRelatedJobLogsResponse {
  return { groups: [], nextPageToken: '' };
}
