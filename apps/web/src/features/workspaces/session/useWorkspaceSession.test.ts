import { renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';
import { useWorkspaceSession, type UseWorkspaceSessionArgs } from './useWorkspaceSession';

const hookState = vi.hoisted(() => ({
  rawJobStatus: undefined as FirestoreJobStatus | undefined,
  workspaceJobs: [] as FirestoreJobStatus[],
}));

vi.mock('@/features/jobs/firestore/useJobStatus', () => ({
  useJobStatus: () => ({ status: hookState.rawJobStatus, error: null }),
}));

vi.mock('@/features/jobs/firestore/useWorkspaceJobStatuses', () => ({
  useWorkspaceJobStatuses: () => ({ jobs: hookState.workspaceJobs }),
}));

vi.mock('@/lib/observability/log', () => ({
  log: {
    error: vi.fn(),
  },
}));

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

function makeArgs(overrides: Partial<UseWorkspaceSessionArgs> = {}): UseWorkspaceSessionArgs {
  return {
    workspaceId: 'ws_1',
    workspaceName: 'Workspace',
    hasTree: true,
    childItemsCount: 0,
    onUploadFile: vi.fn(),
    onRenameWorkspace: vi.fn(),
    onSuggestedWorkspaceName: vi.fn(),
    onProcessingComplete: vi.fn(),
    ...overrides,
  };
}

describe('useWorkspaceSession', () => {
  beforeEach(() => {
    hookState.rawJobStatus = undefined;
    hookState.workspaceJobs = [];
  });

  it('recovers a completed job after reload when its paper is not projected yet', async () => {
    const onProcessingComplete = vi.fn();
    hookState.workspaceJobs = [makeJob()];

    renderHook(() => useWorkspaceSession(makeArgs({ onProcessingComplete })));

    await waitFor(() => {
      expect(onProcessingComplete).toHaveBeenCalledWith('job_1');
    });
  });

  it('does not replay a completed job when the workspace already has paper nodes', async () => {
    const onProcessingComplete = vi.fn();
    hookState.workspaceJobs = [makeJob()];

    renderHook(() => useWorkspaceSession(makeArgs({
      childItemsCount: 1,
      onProcessingComplete,
    })));

    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(onProcessingComplete).not.toHaveBeenCalled();
  });
});
