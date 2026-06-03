import { hasTreeChange, isJobFailed, isJobInFlight } from '@/features/jobs/contracts/jobStatusContract';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';
import { WORKSPACE_COMPLETION_MESSAGE, type WorkspaceSessionState } from './workspaceSessionTypes';

export function isWorkspaceJobRunning(state: WorkspaceSessionState, jobStatus: FirestoreJobStatus | undefined): boolean {
  return !!state.job.activeJobId && isJobInFlight(jobStatus);
}

export function isWorkspaceJobFailed(jobStatus: FirestoreJobStatus | undefined): boolean {
  return isJobFailed(jobStatus);
}

export function isWorkspaceJustCompleted(state: WorkspaceSessionState): boolean {
  return state.upload.message === WORKSPACE_COMPLETION_MESSAGE;
}

export function didJobChangeTree(jobStatus: FirestoreJobStatus | undefined): boolean {
  return hasTreeChange(jobStatus);
}
