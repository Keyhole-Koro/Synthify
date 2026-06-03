import { describe, expect, it } from 'vitest';
import {
  createWorkspaceSessionState,
  WORKSPACE_COMPLETION_MESSAGE,
  WORKSPACE_UPLOAD_STARTED_MESSAGE,
} from './workspaceSessionTypes';
import { workspaceSessionReducer } from './workspaceSessionReducer';

describe('workspaceSessionReducer', () => {
  it('adopts an in-flight job after reload', () => {
    const initial = createWorkspaceSessionState({
      workspaceId: 'ws_1',
      workspaceName: 'Workspace',
    });

    const state = workspaceSessionReducer(initial, {
      type: 'job/adopted',
      jobId: 'job_1',
      reason: 'inFlight',
    });

    expect(state.phase).toBe('processing');
    expect(state.job.activeJobId).toBe('job_1');
  });

  it('tracks a completed job and shows the completion message', () => {
    const initial = createWorkspaceSessionState({
      workspaceId: 'ws_1',
      workspaceName: 'Workspace',
      initialActiveJobId: 'job_1',
    });

    const state = workspaceSessionReducer(initial, {
      type: 'job/succeeded',
      jobId: 'job_1',
    });

    expect(state.phase).toBe('ready');
    expect(state.job.completedJobIds.job_1).toBe(true);
    expect(state.upload.message).toBe(WORKSPACE_COMPLETION_MESSAGE);
  });

  it('moves upload state from started to enqueued', () => {
    const initial = createWorkspaceSessionState({
      workspaceId: 'ws_1',
      workspaceName: 'Workspace',
    });

    const uploading = workspaceSessionReducer(initial, { type: 'upload/started' });
    expect(uploading.phase).toBe('uploading');
    expect(uploading.upload.uploading).toBe(true);
    expect(uploading.upload.message).toBeNull();

    const enqueued = workspaceSessionReducer(uploading, {
      type: 'upload/enqueued',
      jobId: 'job_1',
      documentId: 'doc_1',
    });
    expect(enqueued.phase).toBe('processing');
    expect(enqueued.job.activeJobId).toBe('job_1');
    expect(enqueued.upload.uploading).toBe(false);
    expect(enqueued.upload.message).toBe(WORKSPACE_UPLOAD_STARTED_MESSAGE);
  });

  it('keeps rename draft in reducer state', () => {
    const initial = createWorkspaceSessionState({
      workspaceId: 'ws_1',
      workspaceName: 'Workspace',
    });

    const editing = workspaceSessionReducer(initial, {
      type: 'rename/started',
      currentName: 'Workspace',
    });
    const changed = workspaceSessionReducer(editing, {
      type: 'rename/inputChanged',
      value: 'Renamed',
    });
    const saved = workspaceSessionReducer(changed, {
      type: 'rename/saveSucceeded',
      name: 'Renamed',
    });

    expect(saved.rename.editing).toBe(false);
    expect(saved.rename.draftName).toBe('Renamed');
    expect(saved.rename.error).toBeNull();
  });

  it('syncs workspace name props only when not editing', () => {
    const initial = createWorkspaceSessionState({
      workspaceId: 'ws_1',
      workspaceName: 'Workspace',
    });
    const editing = workspaceSessionReducer(initial, {
      type: 'rename/started',
      currentName: 'Workspace',
    });
    const changed = workspaceSessionReducer(editing, {
      type: 'rename/inputChanged',
      value: 'Draft',
    });

    const whileEditing = workspaceSessionReducer(changed, {
      type: 'workspace/propsSynced',
      workspaceName: 'Server Name',
    });
    expect(whileEditing.rename.draftName).toBe('Draft');

    const cancelled = workspaceSessionReducer(whileEditing, {
      type: 'rename/cancelled',
      currentName: 'Server Name',
    });
    expect(cancelled.rename.draftName).toBe('Server Name');
  });
});
