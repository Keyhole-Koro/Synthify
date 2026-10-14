import { hasTreeChange, isJobFailed, isJobInFlight } from '@/features/jobs/contracts/jobStatusContract';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';
import { WORKSPACE_COMPLETION_MESSAGE, type WorkspaceSessionState } from './workspaceSessionTypes';

/**
 * Evaluates whether the workspace's active job is currently executing.
 *
 * @param state - Current workspace session state.
 * @param jobStatus - Latest Firestore job status record, if available.
 * @returns True if the job is active and in flight.
 */
export function isWorkspaceJobRunning(state: WorkspaceSessionState, jobStatus: FirestoreJobStatus | undefined): boolean {
  return !!state.job.activeJobId && isJobInFlight(jobStatus);
}

/**
 * Checks whether the given job terminated in a failed state.
 *
 * @param jobStatus - Latest Firestore job status record.
 * @returns True if the job failed.
 */
export function isWorkspaceJobFailed(jobStatus: FirestoreJobStatus | undefined): boolean {
  return isJobFailed(jobStatus);
}

/**
 * Checks whether the workspace analysis completed within the recent TTL window.
 *
 * @param state - Current workspace session state.
 * @returns True if the completion notification is actively showing.
 */
export function isWorkspaceJustCompleted(state: WorkspaceSessionState): boolean {
  return state.upload.message === WORKSPACE_COMPLETION_MESSAGE;
}

/**
 * Determines whether the job produced modifications to the workspace knowledge tree.
 *
 * @param jobStatus - Latest Firestore job status record.
 * @returns True if the job modified tree structure.
 */
export function didJobChangeTree(jobStatus: FirestoreJobStatus | undefined): boolean {
  return hasTreeChange(jobStatus);
}
