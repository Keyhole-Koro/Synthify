import { hasCreatedDocumentRoot, isJobFailed, isJobInFlight } from '@/features/jobs/contract/jobStatusContract';
import type { FirestoreJobStatus } from '@/features/jobs/useJobStatus';
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

export function getCompletedDocumentRootItemId(jobStatus: FirestoreJobStatus | undefined): string | undefined {
  return hasCreatedDocumentRoot(jobStatus) ? jobStatus.createdDocumentRootItemId : undefined;
}
