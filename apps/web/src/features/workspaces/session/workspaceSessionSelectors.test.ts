import { describe, expect, it } from 'vitest';
import { createWorkspaceSessionState, WORKSPACE_COMPLETION_MESSAGE } from './workspaceSessionTypes';
import {
  didJobChangeTree,
  isWorkspaceJobFailed,
  isWorkspaceJobRunning,
  isWorkspaceJustCompleted,
} from './workspaceSessionSelectors';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';

function job(status: FirestoreJobStatus['status'], treeChanged?: boolean): FirestoreJobStatus {
  return {
    jobId: 'job_1',
    jobType: 'JOB_TYPE_PROCESS_DOCUMENT',
    documentId: 'doc_1',
    workspaceId: 'ws_1',
    treeId: 'ws_1',
    status,
    currentStage: '',
    errorMessage: '',
    updatedAt: '2026-01-01T00:00:00Z',
    treeChanged,
  };
}

describe('workspace session selectors', () => {
  it('detects an active job only when state has an active job', () => {
    const state = createWorkspaceSessionState({ workspaceId: 'ws_1', workspaceName: 'Workspace', initialActiveJobId: 'job_1' });
    expect(isWorkspaceJobRunning(state, job('running'))).toBe(true);
    expect(isWorkspaceJobRunning({ ...state, job: { ...state.job, activeJobId: null } }, job('running'))).toBe(false);
    expect(isWorkspaceJobRunning(state, job('succeeded'))).toBe(false);
  });

  it('classifies failed, completed, and tree-changing jobs', () => {
    const state = createWorkspaceSessionState({
      workspaceId: 'ws_1',
      workspaceName: 'Workspace',
      initialUploadMessage: WORKSPACE_COMPLETION_MESSAGE,
    });
    expect(isWorkspaceJobFailed(job('failed'))).toBe(true);
    expect(isWorkspaceJobFailed(job('running'))).toBe(false);
    expect(isWorkspaceJustCompleted(state)).toBe(true);
    expect(didJobChangeTree(job('succeeded', true))).toBe(true);
    expect(didJobChangeTree(job('failed', true))).toBe(false);
  });
});
