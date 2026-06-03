import { create } from '@bufbuild/protobuf';
import type { AuthUser } from '@/features/auth/session';
import { WorkspaceSchema, type Workspace } from '@/gen/proto/synthify/app/v1/workspace_pb';

const STORAGE_KEY = 'synthify_workspace_snapshot';

type WorkspaceSnapshot = {
  workspaceId: string;
  name: string;
  ownerId: string;
  plan: number;
  storageUsedBytes: string;
  storageQuotaBytes: string;
  maxFileSizeBytes: string;
  maxUploadsPerDay: string;
  createdAt: string;
};

function storageKey(owner: AuthUser | null) {
  return `${STORAGE_KEY}:${owner?.id ?? 'anonymous'}`;
}

function toSnapshot(workspace: Workspace): WorkspaceSnapshot {
  return {
    workspaceId: workspace.workspaceId,
    name: workspace.name,
    ownerId: workspace.ownerId,
    plan: workspace.plan,
    storageUsedBytes: workspace.storageUsedBytes.toString(),
    storageQuotaBytes: workspace.storageQuotaBytes.toString(),
    maxFileSizeBytes: workspace.maxFileSizeBytes.toString(),
    maxUploadsPerDay: workspace.maxUploadsPerDay.toString(),
    createdAt: workspace.createdAt,
  };
}

function fromSnapshot(snapshot: WorkspaceSnapshot): Workspace {
  return create(WorkspaceSchema, {
    workspaceId: snapshot.workspaceId,
    name: snapshot.name,
    ownerId: snapshot.ownerId,
    plan: snapshot.plan,
    storageUsedBytes: BigInt(snapshot.storageUsedBytes || '0'),
    storageQuotaBytes: BigInt(snapshot.storageQuotaBytes || '0'),
    maxFileSizeBytes: BigInt(snapshot.maxFileSizeBytes || '0'),
    maxUploadsPerDay: BigInt(snapshot.maxUploadsPerDay || '0'),
    createdAt: snapshot.createdAt,
  });
}

export function loadWorkspaceSnapshot(owner: AuthUser | null): Workspace[] {
  if (typeof window === 'undefined') return [];
  try {
    const serialized = window.localStorage.getItem(storageKey(owner));
    if (!serialized) return [];
    const snapshots = JSON.parse(serialized) as WorkspaceSnapshot[];
    if (!Array.isArray(snapshots)) return [];
    return snapshots
      .filter((snapshot) => snapshot && typeof snapshot.workspaceId === 'string' && snapshot.workspaceId)
      .map(fromSnapshot);
  } catch (err) {
    console.warn('Failed to load workspace snapshot', err);
    return [];
  }
}

export function saveWorkspaceSnapshot(owner: AuthUser | null, workspaces: Workspace[]) {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(
      storageKey(owner),
      JSON.stringify(workspaces.map(toSnapshot)),
    );
  } catch (err) {
    console.warn('Failed to save workspace snapshot', err);
  }
}

export function clearWorkspaceSnapshot(owner: AuthUser | null) {
  if (typeof window === 'undefined') return;
  window.localStorage.removeItem(storageKey(owner));
}
