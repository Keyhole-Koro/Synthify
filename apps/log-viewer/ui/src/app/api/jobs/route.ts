import { NextResponse } from 'next/server';
import { getPool } from '@/lib/db';
import type { ListJobsResponse } from '@/types';

// GET /api/jobs - 全 job 列挙 (admin / log-viewer 用途)
export async function GET() {
  try {
    const pool = getPool();
    const { rows } = await pool.query(`
      SELECT
        job_id,
        document_id,
        workspace_id,
        job_type,
        status,
        current_stage,
        error_message,
        retry_count,
        created_at,
        updated_at
      FROM v_processing_jobs
      ORDER BY created_at DESC
      LIMIT 500
    `);
    const response: ListJobsResponse = {
      jobs: rows.map((r) => ({
        jobId: r.job_id,
        documentId: r.document_id,
        workspaceId: r.workspace_id,
        jobType: r.job_type,
        status: r.status,
        currentStage: r.current_stage,
        errorMessage: r.error_message,
        retryCount: r.retry_count,
        createdAt: r.created_at instanceof Date ? r.created_at.toISOString() : String(r.created_at),
        updatedAt: r.updated_at instanceof Date ? r.updated_at.toISOString() : String(r.updated_at),
      })),
    };
    return NextResponse.json(response);
  } catch (error) {
    console.error('log-viewer jobs list failed:', error);
    const response: ListJobsResponse = { jobs: [] };
    return NextResponse.json(response, { status: 500 });
  }
}
