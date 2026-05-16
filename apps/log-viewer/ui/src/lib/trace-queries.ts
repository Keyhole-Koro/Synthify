import type { Pool } from 'pg';

export interface ToolCallEntry {
  mutationId: string;
  toolName: string;
  durationMs: number;
  inputJson: string;
  outputJson: string;
  isError: boolean;
  createdAt: string;
}

export interface JobTraceData {
  jobId: string;
  toolCalls: ToolCallEntry[];
}

export async function queryJobTrace(pool: Pool, jobId: string): Promise<JobTraceData> {
  const { rows } = await pool.query(
    `SELECT mutation_id, target_id, before_json, after_json, provenance_json, created_at
     FROM v_job_mutation_logs
     WHERE job_id = $1 AND target_type = 'tool_call'
     ORDER BY created_at ASC`,
    [jobId],
  );

  const toolCalls: ToolCallEntry[] = rows.map((r) => {
    const provenance = parseJson(String(r.provenance_json));
    const outputJson = String(r.after_json);
    const output = parseJson(outputJson);
    const isError = typeof output === 'object' && output !== null && 'error' in output;

    return {
      mutationId: String(r.mutation_id),
      toolName: String(r.target_id),
      durationMs: typeof provenance?.duration_ms === 'number' ? provenance.duration_ms : 0,
      inputJson: String(r.before_json),
      outputJson,
      isError,
      createdAt: r.created_at instanceof Date ? r.created_at.toISOString() : String(r.created_at),
    };
  });

  return { jobId, toolCalls };
}

function parseJson(s: string): Record<string, unknown> | null {
  try {
    return JSON.parse(s) as Record<string, unknown>;
  } catch {
    return null;
  }
}
