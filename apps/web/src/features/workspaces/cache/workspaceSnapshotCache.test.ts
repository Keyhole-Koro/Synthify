import { create } from '@bufbuild/protobuf';
import { describe, expect, it, beforeEach } from 'vitest';
import { WorkspacePlan, WorkspaceSchema } from '@/gen/proto/synthify/app/v1/workspace_pb';
import { loadWorkspaceSnapshot, saveWorkspaceSnapshot } from './workspaceSnapshotCache';

const owner = { id: 'user_1', email: 'user@example.com', name: 'User' };

describe('workspaceSnapshotCache', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('round-trips workspace snapshots with bigint fields', () => {
    const workspace = create(WorkspaceSchema, {
      workspaceId: 'ws_1',
      name: 'Cached workspace',
      ownerId: 'account_1',
      plan: WorkspacePlan.FREE,
      storageUsedBytes: 12n,
      storageQuotaBytes: 34n,
      maxFileSizeBytes: 56n,
      maxUploadsPerDay: 78n,
      createdAt: '2026-06-03T00:00:00Z',
    });

    saveWorkspaceSnapshot(owner, [workspace]);

    const [loaded] = loadWorkspaceSnapshot(owner);
    expect(loaded.workspaceId).toBe('ws_1');
    expect(loaded.name).toBe('Cached workspace');
    expect(loaded.storageQuotaBytes).toBe(34n);
    expect(loaded.maxUploadsPerDay).toBe(78n);
  });

  it('scopes cached workspaces by user id', () => {
    const workspace = create(WorkspaceSchema, {
      workspaceId: 'ws_1',
      name: 'Cached workspace',
    });

    saveWorkspaceSnapshot(owner, [workspace]);

    expect(loadWorkspaceSnapshot({ ...owner, id: 'user_2' })).toEqual([]);
  });
});
