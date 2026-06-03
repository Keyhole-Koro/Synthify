import { describe, expect, it } from 'vitest';
import type { FirestoreJobStatus } from '@/features/jobs/useJobStatus';
import {
  hasCreatedDocumentRoot,
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

  it('requires a document root id for completed document-root recovery', () => {
    expect(hasCreatedDocumentRoot(makeJob({
      status: 'succeeded',
      createdDocumentRootItemId: 'doc_root_1',
    }))).toBe(true);
    expect(hasCreatedDocumentRoot(makeJob({ status: 'succeeded' }))).toBe(false);
    expect(hasCreatedDocumentRoot(makeJob({
      status: 'failed',
      createdDocumentRootItemId: 'doc_root_1',
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
