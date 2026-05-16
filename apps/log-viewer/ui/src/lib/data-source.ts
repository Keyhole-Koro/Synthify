// log-viewer はクライアント側からは API (Connect/gRPC) を叩かず、
// 同一 Next.js アプリの Route Handlers (/api/jobs/...) を経由して
// サーバーサイドで Postgres を read-only ロールで直接参照する。

import type {
  JobLogDataSource,
  JobLogFilters,
  ListJobLogsResponse,
  ListRelatedJobLogsRequest,
  ListRelatedJobLogsResponse,
  SearchJobLogsRequest,
} from '../types';

export function createJobLogDataSource(): JobLogDataSource {
  return {
    async listJobLogs(jobId, pageToken) {
      const qs = pageToken ? `?page_token=${encodeURIComponent(pageToken)}` : '';
      const res = await fetch(`/api/jobs/${encodeURIComponent(jobId)}/logs${qs}`);
      if (!res.ok) return { logs: [], nextPageToken: '' };
      const body = (await res.json()) as Partial<ListJobLogsResponse>;
      return {
        logs: Array.isArray(body.logs) ? body.logs : [],
        nextPageToken: body.nextPageToken ?? '',
      };
    },
    async searchJobLogs(filters, ids, pageToken) {
      const requestBody: SearchJobLogsRequest = {
        query: filters.query,
        workspaceId: filters.scope === 'workspace' ? (ids.workspaceId ?? '') : '',
        documentId: filters.scope === 'document' ? (ids.documentId ?? '') : '',
        jobId: filters.scope === 'job' ? ids.jobId : '',
        levels: filters.levels,
        events: filters.events,
        fromTimestamp: filters.fromTimestamp ?? '',
        toTimestamp: filters.toTimestamp ?? '',
        pageToken: pageToken ?? '',
      };
      const res = await fetch('/api/jobs/search', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestBody),
      });
      if (!res.ok) return { logs: [], nextPageToken: '' };
      const body = (await res.json()) as Partial<ListJobLogsResponse>;
      return {
        logs: Array.isArray(body.logs) ? body.logs : [],
        nextPageToken: body.nextPageToken ?? '',
      };
    },
    async listRelatedJobLogs(scope, ids, pageToken) {
      const requestBody: ListRelatedJobLogsRequest = {
        scope,
        jobId: scope === 'job' ? ids.jobId : '',
        documentId: scope === 'document' ? (ids.documentId ?? '') : '',
        workspaceId: scope === 'workspace' ? (ids.workspaceId ?? '') : '',
        pageToken: pageToken ?? '',
      };
      const res = await fetch('/api/jobs/related', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestBody),
      });
      if (!res.ok) return { groups: [], nextPageToken: '' };
      const body = (await res.json()) as Partial<ListRelatedJobLogsResponse>;
      return {
        groups: Array.isArray(body.groups) ? body.groups : [],
        nextPageToken: body.nextPageToken ?? '',
      };
    },
  };
}

// JobLogFilters はクライアントから受け取った値をそのまま転送するだけなので
// 直接の参照は不要だが、tree-shaking と型安全のため明示的に再エクスポート。
export type { JobLogFilters };
