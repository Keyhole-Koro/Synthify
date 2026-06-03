import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';
import { hasTreeChange, isJobInFlight } from '@/features/jobs/contracts/jobStatusContract';

export interface WorkspaceRecoveryState {
  hasActiveJob: boolean;
  hasTree: boolean;
  childItemsCount: number;
  completedJobIds?: Record<string, true>;
}

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
