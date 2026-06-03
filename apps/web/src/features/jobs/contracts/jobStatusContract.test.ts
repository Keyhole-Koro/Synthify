import { describe, expect, it } from 'vitest';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';
import {
  hasTreeChange,
  isJobFailed,
  isJobInFlight,
  isJobSucceeded,
  isJobTerminal,
  jobStatusLabel,
  jobStatusToneClass,
} from './jobStatusContract';

function makeJob(overrides: Partial<FirestoreJobStatus>): FirestoreJobStatus {
  return {
    jobId: 'job_1',
    jobType: 'JOB_TYPE_PROCESS_DOCUMENT',
    documentId: 'doc_1',
    workspaceId: 'ws_1',
    treeId: 'ws_1',
    status: 'queued',
    currentStage: '',
    errorMessage: '',
    updatedAt: '2026-06-03T00:00:00Z',
    ...overrides,
  };
}

describe('job status contract', () => {
  it('classifies in-flight states', () => {
    expect(isJobInFlight(makeJob({ status: 'queued' }))).toBe(true);
    expect(isJobInFlight(makeJob({ status: 'running' }))).toBe(true);
    expect(isJobInFlight(makeJob({ status: 'succeeded' }))).toBe(false);
    expect(isJobInFlight(makeJob({ status: 'failed' }))).toBe(false);
  });

  it('classifies terminal states', () => {
    expect(isJobTerminal(makeJob({ status: 'queued' }))).toBe(false);
    expect(isJobTerminal(makeJob({ status: 'running' }))).toBe(false);
    expect(isJobTerminal(makeJob({ status: 'succeeded' }))).toBe(true);
    expect(isJobTerminal(makeJob({ status: 'failed' }))).toBe(true);
  });

  it('requires treeChanged for completed tree-change recovery', () => {
    expect(hasTreeChange(makeJob({
      status: 'succeeded',
      treeChanged: true,
    }))).toBe(true);
    expect(hasTreeChange(makeJob({ status: 'succeeded' }))).toBe(false);
    expect(hasTreeChange(makeJob({
      status: 'failed',
      treeChanged: true,
    }))).toBe(false);
  });

  it('labels and styles status for workspace job receipts', () => {
    const succeeded = makeJob({ status: 'succeeded' });
    const failed = makeJob({ status: 'failed' });

    expect(isJobSucceeded(succeeded)).toBe(true);
    expect(isJobFailed(failed)).toBe(true);
    expect(jobStatusLabel(succeeded)).toBe('完了');
    expect(jobStatusToneClass(failed)).toContain('bg-red-500');
  });
});
