import { queryOptions } from '@tanstack/react-query';
import {
  getTree,
  getTreeViaShareToken,
  getSubtree,
  getTreeEntityDetail,
  listAllJobs,
} from './api';
import type { TreeProjectionScope } from './api';

/**
 * Builds the cache key for the full workspace tree.
 * @param workspaceId - Workspace whose tree is being queried.
 * @returns A stable workspace-scoped query key.
 */
export const workspaceTreeQueryKey = (workspaceId: string) => ['tree', workspaceId] as const;

/**
 * Builds the cache key for a tree retrieved through a public share token.
 * @param workspaceId - Workspace referenced by the public link.
 * @param shareToken - Token that grants read access to the shared workspace.
 * @returns A key isolated from authenticated tree queries and other share tokens.
 */
export const shareTokenTreeQueryKey = (workspaceId: string, shareToken: string) =>
  ['tree', workspaceId, 'share', shareToken] as const;

/**
 * Builds the cache key for a subtree rooted at one item.
 * @param workspaceId - Workspace containing the item.
 * @param itemId - Root item identifier.
 * @param maxDepth - Maximum traversal depth included in the response.
 * @returns A key that changes whenever the workspace, root item, or depth changes.
 */
export const subtreeQueryKey = (workspaceId: string, itemId: string, maxDepth = 3) =>
  ['subtree', workspaceId, itemId, maxDepth] as const;

/**
 * Builds the cache key for a detailed tree entity query.
 * @param targetRef - Workspace, projection scope, entity ID, and optional document ID.
 * @param resolveAliases - Whether alias resolution is enabled for this query.
 * @returns A key including every input that can change the returned entity detail.
 */
export const entityDetailQueryKey = (
  targetRef: { workspaceId: string; scope: TreeProjectionScope; id: string; documentId?: string },
  resolveAliases = false,
) => ['entity-detail', targetRef.workspaceId, targetRef.scope, targetRef.id, targetRef.documentId, resolveAliases] as const;

/** Query key shared by the global job-monitoring query. */
export const jobsQueryKey = ['jobs'] as const;

/**
 * Creates TanStack Query options for retrieving the full workspace tree.
 * @param workspaceId - Workspace whose tree should be loaded.
 * @returns Query options with a 15-second stale time and disabled execution for an empty ID.
 */
export function workspaceTreeQueryOptions(workspaceId: string) {
  return queryOptions({
    queryKey: workspaceTreeQueryKey(workspaceId),
    queryFn: () => getTree(workspaceId),
    staleTime: 15_000,
    enabled: Boolean(workspaceId),
  });
}

/**
 * Creates TanStack Query options for retrieving a tree via share token.
 * @param workspaceId - Workspace referenced by the public link.
 * @param shareToken - Unauthenticated access token.
 * @returns Query options with a one-minute stale time, disabled until both identifiers exist.
 */
export function shareTokenTreeQueryOptions(workspaceId: string, shareToken: string) {
  return queryOptions({
    queryKey: shareTokenTreeQueryKey(workspaceId, shareToken),
    queryFn: () => getTreeViaShareToken(workspaceId, shareToken),
    staleTime: 60_000,
    enabled: Boolean(workspaceId && shareToken),
  });
}

/**
 * Creates TanStack Query options for querying a subtree.
 * @param workspaceId - Workspace containing the subtree.
 * @param itemId - Subtree root item identifier.
 * @param maxDepth - Maximum depth passed to the API.
 * @returns Query options with a 30-second stale time and input-dependent cache key.
 */
export function subtreeQueryOptions(workspaceId: string, itemId: string, maxDepth = 3) {
  return queryOptions({
    queryKey: subtreeQueryKey(workspaceId, itemId, maxDepth),
    queryFn: () => getSubtree(workspaceId, itemId, maxDepth),
    staleTime: 30_000,
    enabled: Boolean(workspaceId && itemId),
  });
}

/**
 * Creates TanStack Query options for retrieving entity details.
 * @param targetRef - Reference identifying the entity and projection scope.
 * @param resolveAliases - Whether to resolve aliases before returning details.
 * @returns Query options disabled until the workspace and entity IDs are available.
 */
export function entityDetailQueryOptions(
  targetRef: { workspaceId: string; scope: TreeProjectionScope; id: string; documentId?: string },
  resolveAliases = false,
) {
  return queryOptions({
    queryKey: entityDetailQueryKey(targetRef, resolveAliases),
    queryFn: () => getTreeEntityDetail(targetRef, resolveAliases),
    staleTime: 30_000,
    enabled: Boolean(targetRef.workspaceId && targetRef.id),
  });
}

/**
 * TanStack Query options for monitoring all jobs.
 * The key is global because the API applies the authenticated user's visibility scope.
 */
export const jobsQueryOptions = queryOptions({
  queryKey: jobsQueryKey,
  queryFn: listAllJobs,
  staleTime: 10_000,
});
