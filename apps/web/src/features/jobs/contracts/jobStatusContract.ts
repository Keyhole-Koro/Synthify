import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';

/**
 * Succeeded job record that modified the workspace knowledge tree.
 *
 * @property status Always `succeeded` for this narrowed record.
 * @property treeChanged Always `true`, indicating that consumers should refresh the tree.
 * @property jobId, documentId, workspaceId, and other fields Inherited Firestore job metadata.
 */
export type CompletedJobWithTreeChange = FirestoreJobStatus & {
  status: 'succeeded';
  treeChanged: true;
};

type SucceededJob = FirestoreJobStatus & { status: 'succeeded' };
type FailedJob = FirestoreJobStatus & { status: 'failed' };

/**
 * Checks whether a job status is in the `queued` state.
 * @param job - Firestore status snapshot, or undefined before the first snapshot arrives.
 * @returns True only when the snapshot explicitly reports `queued`.
 */
export function isJobQueued(job: FirestoreJobStatus | undefined): boolean {
  return job?.status === 'queued';
}

/**
 * Checks whether a job status is actively `running`.
 * @param job - Firestore status snapshot, or undefined when no status is available.
 * @returns True only for an actively running job.
 */
export function isJobRunning(job: FirestoreJobStatus | undefined): boolean {
  return job?.status === 'running';
}

/**
 * Checks whether a job is currently in flight (either queued or running).
 * @param job - Firestore status snapshot to classify.
 * @returns True for queued/running jobs; false for missing and terminal statuses.
 */
export function isJobInFlight(job: FirestoreJobStatus | undefined): boolean {
  return isJobQueued(job) || isJobRunning(job);
}

/**
 * Narrows a status snapshot to a succeeded job.
 * @param job - Firestore status snapshot to inspect.
 * @returns True when `job` exists and its status is `succeeded`.
 */
export function isJobSucceeded(job: FirestoreJobStatus | undefined): job is SucceededJob {
  return job?.status === 'succeeded';
}

/**
 * Narrows a status snapshot to a failed job.
 * @param job - Firestore status snapshot to inspect.
 * @returns True when `job` exists and its status is `failed`.
 */
export function isJobFailed(job: FirestoreJobStatus | undefined): job is FailedJob {
  return job?.status === 'failed';
}

/**
 * Checks whether a job has reached a terminal state (either succeeded or failed).
 * @param job - Firestore status snapshot to classify.
 * @returns True for succeeded or failed records; false while queued/running or missing.
 */
export function isJobTerminal(job: FirestoreJobStatus | undefined): boolean {
  return isJobSucceeded(job) || isJobFailed(job);
}

/**
 * Narrows a job to a successful run that changed the knowledge tree.
 * @param job - Firestore status snapshot to inspect.
 * @returns True only when the job succeeded and `treeChanged` is explicitly true.
 */
export function hasTreeChange(job: FirestoreJobStatus | undefined): job is CompletedJobWithTreeChange {
  return isJobSucceeded(job) && job.treeChanged === true;
}

/**
 * Returns a human-friendly Japanese status label for a job.
 * @param job - Complete Firestore job status record.
 * @returns Localized label for the known status, or the raw status for forward compatibility.
 */
export function jobStatusLabel(job: FirestoreJobStatus): string {
  switch (job.status) {
    case 'succeeded':
      return '完了';
    case 'failed':
      return '失敗';
    case 'running':
      return '実行中';
    case 'queued':
      return '待機中';
    default:
      return job.status;
  }
}

/**
 * Returns Tailwind CSS classes for a job status indicator pill.
 * @param job - Complete Firestore job status record.
 * @returns Background/animation classes used by the status badge; unknown statuses receive
 *   the neutral fallback class.
 */
export function jobStatusToneClass(job: FirestoreJobStatus): string {
  switch (job.status) {
    case 'succeeded':
      return 'bg-emerald-500';
    case 'failed':
      return 'bg-red-500';
    case 'running':
      return 'bg-indigo-500 animate-pulse';
    case 'queued':
      return 'bg-amber-400';
    default:
      return 'bg-stone-300';
  }
}
