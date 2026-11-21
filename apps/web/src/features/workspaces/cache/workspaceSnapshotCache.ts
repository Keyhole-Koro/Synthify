import { create } from '@bufbuild/protobuf';
import type { AuthUser } from '@/features/auth/session';
import { WorkspaceSchema, type Workspace } from '@/gen/proto/synthify/app/v1/workspace_pb';

const STORAGE_KEY = 'synthify_workspace_snapshot';

/**
 * JSON-serializable snapshot representation of a workspace entity,
 * safely serializing BigInt fields into strings for localStorage persistence.
 */
type WorkspaceSnapshot = {
  /** Protobuf workspace identifier. */
  workspaceId: string;
  /** User-facing workspace name. */
  name: string;
  /** Account/user identifier that owns the workspace. */
  ownerId: string;
  /** Numeric protobuf plan enum value. */
  plan: number;
  /** BigInt storage usage serialized as decimal text for JSON compatibility. */
  storageUsedBytes: string;
  /** BigInt storage quota serialized as decimal text. */
  storageQuotaBytes: string;
  /** BigInt per-file limit serialized as decimal text. */
  maxFileSizeBytes: string;
  /** BigInt daily upload limit serialized as decimal text. */
  maxUploadsPerDay: string;
  /** ISO-8601 workspace creation timestamp. */
  createdAt: string;
};

/**
 * Computes a scoped storage key for workspace caching based on the user ID.
 */
function storageKey(owner: AuthUser | null) {
  return `${STORAGE_KEY}:${owner?.id ?? 'anonymous'}`;
}

/**
 * Converts a Protobuf Workspace instance into a serializable snapshot.
 */
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

/**
 * Reconstitutes a Protobuf Workspace instance from a serialized snapshot.
 */
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

/**
 * Loads the cached list of workspaces for the given user from localStorage.
 *
 * @param owner - The authenticated user or null.
 * @returns Array of restored workspace entities, or empty if none found or corrupted.
 */
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

/**
 * Persists the current list of workspaces to localStorage under the user's scope.
 *
 * @param owner - The authenticated user or null.
 * @param workspaces - List of workspace entities to cache.
 * @returns Nothing. Missing browser storage or serialization failures are
 *   treated as best-effort cache misses and do not interrupt the caller.
 */
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

/**
 * Removes the cached workspace snapshot for the given user from localStorage.
 *
 * @param owner - The user whose cache should be purged.
 * @returns Nothing. Missing browser storage is treated as a no-op.
 */
export function clearWorkspaceSnapshot(owner: AuthUser | null) {
  if (typeof window === 'undefined') return;
  window.localStorage.removeItem(storageKey(owner));
}
