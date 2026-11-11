/**
 * Core type definitions and domain interfaces for the Monitor UI application.
 *
 * @module
 */
import type { components } from './generated/openapi';

/** Summarized job execution metadata. */
export type JobSummary = components['schemas']['JobSummary'];
/** Structured job log entry. */
export type JobLog = components['schemas']['JobLog'];
/** Related job execution reference. */
export type JobLogJob = components['schemas']['RelatedJob'];
/** Group of related jobs sharing correlation IDs. */
export type JobLogGroup = components['schemas']['RelatedJobGroup'];
/** Response payload containing paginated job logs. */
export type ListJobLogsResponse = components['schemas']['ListJobLogsResponse'];
/** Response payload containing paginated jobs list. */
export type ListJobsResponse = components['schemas']['ListJobsResponse'];
/** Request parameters for retrieving related job logs. */
export type ListRelatedJobLogsRequest = components['schemas']['ListRelatedJobLogsRequest'];
/** Response payload containing related job log groups. */
export type ListRelatedJobLogsResponse = components['schemas']['ListRelatedJobLogsResponse'];
/** Request payload for full-text and filtered log search. */
export type SearchJobLogsRequest = components['schemas']['SearchJobLogsRequest'];

/**
 * Filter criteria applied when querying job logs.
 */
export interface JobLogFilters {
  /** Full-text search keyword query. */
  query: string;
  /** Severity level filters (e.g. INFO, WARN, ERROR). */
  levels: string[];
  /** Event category name filters. */
  events: string[];
  /** Correlation scope for related job lookup. */
  scope: components['schemas']['RelatedScope'];
  /** Optional lower bound ISO timestamp. */
  fromTimestamp?: string;
  /** Optional upper bound ISO timestamp. */
  toTimestamp?: string;
}
/**
 * Data access abstraction for fetching and searching job logs.
 */
export interface JobLogDataSource {
  /** Retrieves paginated logs for a specific job ID. */
  listJobLogs(jobId: string, pageToken?: string): Promise<ListJobLogsResponse>;
  /** Searches job logs matching specified filter criteria and entity IDs. */
  searchJobLogs(filters: JobLogFilters, ids: { jobId: string; documentId?: string; workspaceId?: string }, pageToken?: string): Promise<ListJobLogsResponse>;
  /** Retrieves related job logs matching correlation scope. */
  listRelatedJobLogs(scope: JobLogFilters['scope'], ids: { jobId: string; documentId?: string; workspaceId?: string }, pageToken?: string): Promise<ListRelatedJobLogsResponse>;
}
