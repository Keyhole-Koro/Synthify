import { describe, expect, it } from 'vitest';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';
import { selectRecoverableWorkspaceJob } from './workspaceRecovery';

function makeJob(overrides: Partial<FirestoreJobStatus> = {}): FirestoreJobStatus {
  return {
    jobId: 'job_1',
    jobType: 'JOB_TYPE_PROCESS_DOCUMENT',
    documentId: 'doc_1',
    workspaceId: 'ws_1',
    treeId: 'ws_1',
    status: 'succeeded',
    currentStage: 'done',
    errorMessage: '',
    updatedAt: '2026-06-03T00:00:00Z',
    treeChanged: true,
    ...overrides,
  };
}

describe('selectRecoverableWorkspaceJob', () => {
  it('prefers an in-flight job so progress can resume after reload', () => {
    const job = makeJob({ jobId: 'job_running', status: 'running' });

    expect(selectRecoverableWorkspaceJob([job], {
      hasActiveJob: false,
      hasTree: true,
      childItemsCount: 1,
    })?.jobId).toBe('job_running');
  });

  it('recovers a completed job when its paper is not projected yet', () => {
    const job = makeJob();

    expect(selectRecoverableWorkspaceJob([job], {
      hasActiveJob: false,
      hasTree: true,
      childItemsCount: 0,
    })?.jobId).toBe('job_1');
  });

  it('does not replay completed jobs once paper nodes are projected', () => {
    expect(selectRecoverableWorkspaceJob([makeJob()], {
      hasActiveJob: false,
      hasTree: true,
      childItemsCount: 1,
    })).toBeUndefined();
  });

  it('does not recover a completed job without a document root item id', () => {
    expect(selectRecoverableWorkspaceJob([
      makeJob({ treeChanged: undefined }),
    ], {
      hasActiveJob: false,
      hasTree: false,
      childItemsCount: 0,
    })).toBeUndefined();
  });

  it('does not recover while an active job is already selected', () => {
    expect(selectRecoverableWorkspaceJob([makeJob({ status: 'running' })], {
      hasActiveJob: true,
      hasTree: false,
      childItemsCount: 0,
    })).toBeUndefined();
  });

  it('does not recover a completed job that was already processed in the session', () => {
    expect(selectRecoverableWorkspaceJob([makeJob()], {
      hasActiveJob: false,
      hasTree: false,
      childItemsCount: 0,
      completedJobIds: { job_1: true },
    })).toBeUndefined();
  });
});
