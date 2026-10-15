import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';
import { hasTreeChange, isJobInFlight } from '@/features/jobs/contracts/jobStatusContract';

/**
 * Contextual state used to determine if an abandoned or in-flight job should be adopted.
 */
export interface WorkspaceRecoveryState {
  /** Whether the workspace already has an active tracking job. */
  hasActiveJob: boolean;
  /** Whether the workspace already has a loaded tree. */
  hasTree: boolean;
  /** Number of child items currently projected. */
  childItemsCount: number;
  /** Set of job IDs already handled to completion. */
  completedJobIds?: Record<string, true>;
}

/**
 * Inspects recent Firestore jobs to detect unmonitored in-flight jobs or completed
 * tree-modifying jobs that require recovery or tree refreshing.
 *
 * @param jobs - List of recent workspace jobs.
 * @param state - Current recovery context.
 * @returns The job status to adopt, or undefined if no recovery action is needed.
 */
export function selectRecoverableWorkspaceJob(
  jobs: FirestoreJobStatus[],
  state: WorkspaceRecoveryState,
): FirestoreJobStatus | undefined {
  if (state.hasActiveJob || jobs.length === 0) return undefined;

  const inFlight = jobs.find(isJobInFlight);
  if (inFlight && state.completedJobIds?.[inFlight.jobId] !== true) return inFlight;

  if (state.hasTree && state.childItemsCount > 0) return undefined;

  const completed = jobs.find(hasTreeChange);
  if (!completed || state.completedJobIds?.[completed.jobId] === true) return undefined;
  return completed;
}
