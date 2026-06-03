import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';

export type CompletedJobWithDocumentRoot = FirestoreJobStatus & {
  status: 'succeeded';
  createdDocumentRootItemId: string;
};

type SucceededJob = FirestoreJobStatus & { status: 'succeeded' };
type FailedJob = FirestoreJobStatus & { status: 'failed' };

export function isJobQueued(job: FirestoreJobStatus | undefined): boolean {
  return job?.status === 'queued';
}

export function isJobRunning(job: FirestoreJobStatus | undefined): boolean {
  return job?.status === 'running';
}

export function isJobInFlight(job: FirestoreJobStatus | undefined): boolean {
  return isJobQueued(job) || isJobRunning(job);
}

export function isJobSucceeded(job: FirestoreJobStatus | undefined): job is SucceededJob {
  return job?.status === 'succeeded';
}

export function isJobFailed(job: FirestoreJobStatus | undefined): job is FailedJob {
  return job?.status === 'failed';
}

export function isJobTerminal(job: FirestoreJobStatus | undefined): boolean {
  return isJobSucceeded(job) || isJobFailed(job);
}

export function hasCreatedDocumentRoot(job: FirestoreJobStatus | undefined): job is CompletedJobWithDocumentRoot {
  return isJobSucceeded(job) && !!job.createdDocumentRootItemId;
}

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
