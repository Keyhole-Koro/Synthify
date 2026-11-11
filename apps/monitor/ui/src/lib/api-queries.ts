/**
 * TanStack Query definitions for the monitor's own /api endpoints.
 *
 * Each factory returns `queryOptions` (key + fetcher), so screens share one
 * cache entry per request and refetch only when the key changes.
 * (The `*-queries.ts` files next to this one are the server-side SQL.)
 *
 * @module
 */
import { queryOptions } from '@tanstack/react-query';
import { authFetch } from './auth-client';
import type { Period } from './dashboard-queries';
import type { EvalData, EvalTraceEvent } from './eval-queries';
import type { JobTraceData } from './trace-queries';
import type { JobSummary, ListJobsResponse } from '../types';

/**
 * Fetches JSON from an authenticated monitor API endpoint.
 *
 * @param url - Endpoint path.
 * @returns Decoded JSON body.
 * @throws Error carrying the response text (or status) when the response is not OK.
 */
export async function fetchJson<T>(url: string): Promise<T> {
  const res = await authFetch(url);
  if (!res.ok) {
    const message = await res.text().catch(() => '');
    throw new Error(message || `Request failed: ${res.status}`);
  }
  return res.json() as Promise<T>;
}

/** Dashboard tabs served by /api/dashboards/<name>?period=. */
export type DashboardName = 'job-health' | 'cost' | 'workspace' | 'errors';

/** One operations dashboard tab for a period. The body is partial until normalized. */
export function dashboardQuery<T>(name: DashboardName, period: Period) {
  return queryOptions({
    queryKey: ['dashboards', name, period],
    queryFn: () => fetchJson<Partial<T>>(`/api/dashboards/${name}?period=${period}`),
  });
}

/** The LLM Eval dashboard for a period. */
export function evalDashboardQuery(period: Period) {
  return queryOptions({
    queryKey: ['dashboards', 'eval', period],
    queryFn: () => fetchJson<Partial<EvalData>>(`/api/dashboards/eval?period=${period}`),
  });
}

/** Recorded trace events of one eval case. A failed lookup reads as "no recorded trace". */
export function evalTraceQuery(runId: string, caseIndex: number) {
  return queryOptions({
    queryKey: ['dashboards', 'eval', 'trace', runId, caseIndex],
    queryFn: async (): Promise<EvalTraceEvent[]> => {
      const res = await authFetch(`/api/dashboards/eval/trace?runId=${encodeURIComponent(runId)}&caseIndex=${caseIndex}`);
      if (!res.ok) return [];
      const body = (await res.json()) as { events?: EvalTraceEvent[] };
      return Array.isArray(body.events) ? body.events : [];
    },
  });
}

/** Tool calls recorded for one job. */
export function jobTraceQuery(jobId: string) {
  return queryOptions({
    queryKey: ['jobs', jobId, 'trace'],
    queryFn: () => fetchJson<JobTraceData>(`/api/jobs/trace?jobId=${encodeURIComponent(jobId)}`),
  });
}

/** Recent and running jobs for the log viewer's job list. */
export function jobsQuery() {
  return queryOptions({
    queryKey: ['jobs'],
    queryFn: async (): Promise<JobSummary[]> => {
      const body = await fetchJson<Partial<ListJobsResponse>>('/api/jobs');
      return Array.isArray(body.jobs) ? body.jobs : [];
    },
  });
}
