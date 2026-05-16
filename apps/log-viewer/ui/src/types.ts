import type { components } from './generated/openapi';

export type JobSummary = components['schemas']['JobSummary'];
export type JobLog = components['schemas']['JobLog'];
export type JobLogJob = components['schemas']['RelatedJob'];
export type JobLogGroup = components['schemas']['RelatedJobGroup'];
export type ListJobLogsResponse = components['schemas']['ListJobLogsResponse'];
export type ListJobsResponse = components['schemas']['ListJobsResponse'];
export type ListRelatedJobLogsRequest = components['schemas']['ListRelatedJobLogsRequest'];
export type ListRelatedJobLogsResponse = components['schemas']['ListRelatedJobLogsResponse'];
export type SearchJobLogsRequest = components['schemas']['SearchJobLogsRequest'];

export interface JobLogFilters {
  query: string;
  levels: string[];
  events: string[];
  scope: components['schemas']['RelatedScope'];
  fromTimestamp?: string;
  toTimestamp?: string;
}

export interface JobLogDataSource {
  listJobLogs(jobId: string, pageToken?: string): Promise<ListJobLogsResponse>;
  searchJobLogs(filters: JobLogFilters, ids: { jobId: string; documentId?: string; workspaceId?: string }, pageToken?: string): Promise<ListJobLogsResponse>;
  listRelatedJobLogs(scope: JobLogFilters['scope'], ids: { jobId: string; documentId?: string; workspaceId?: string }, pageToken?: string): Promise<ListRelatedJobLogsResponse>;
}
