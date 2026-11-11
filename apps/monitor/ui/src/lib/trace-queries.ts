/**
 * PostgreSQL query helpers for reconstructing job tool call execution traces.
 *
 * @module
 */
import type { Pool } from 'pg';

/**
 * Record of an individual tool call mutation span within a job.
 * @property mutationId Mutation-log record identifier.
 * @property toolName Tool or target name associated with the mutation.
 * @property durationMs Recorded duration in milliseconds, or 0 when unavailable.
 * @property inputJson JSON payload before the tool call.
 * @property outputJson JSON payload after the tool call.
 * @property isError Whether the output contains a tool error.
 * @property createdAt ISO-8601 creation timestamp.
 */
export interface ToolCallEntry {
  mutationId: string;
  toolName: string;
  durationMs: number;
  inputJson: string;
  outputJson: string;
  isError: boolean;
  createdAt: string;
}

/**
 * Complete tool invocation trace sequence for a job execution.
 * @property jobId Job identifier used for the query.
 * @property toolCalls Tool calls ordered by their creation timestamp.
 */
export interface JobTraceData {
  jobId: string;
  toolCalls: ToolCallEntry[];
}

/**
 * Queries tool call mutation records for a given job, reconstructing execution timeline and input/output payloads.
 *
 * @param pool PostgreSQL client pool.
 * @param jobId Target job UUID.
 * @returns Promise resolving to JobTraceData.
 */
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

/** Safely parses a JSON string into an object, returning null on syntax errors. */
function parseJson(s: string): Record<string, unknown> | null {
  try {
    return JSON.parse(s) as Record<string, unknown>;
  } catch {
    return null;
  }
}
