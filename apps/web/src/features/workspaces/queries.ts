import { queryOptions } from '@tanstack/react-query';
import { signInUser } from '@/features/auth/userApi';
import {
  listWorkspaces,
  getWorkspace,
  listShareLinks,
  resolveShareLink,
} from './api';

/**
 * Constructs the TanStack query key for listing workspaces.
 *
 * @param userId - Optional user identifier to namespace the cache key.
 * @returns Query key tuple for workspace list.
 */
export const workspacesQueryKey = (userId?: string | null) =>
  userId ? (['workspaces', userId] as const) : (['workspaces'] as const);

/**
 * Constructs the TanStack query key for a single workspace's details.
 *
 * @param workspaceId - Workspace identifier.
 * @returns Query key tuple for workspace details.
 */
export const workspaceDetailQueryKey = (workspaceId: string) => ['workspaces', workspaceId] as const;

/**
 * Constructs the TanStack query key for a workspace's public share links.
 *
 * @param workspaceId - Workspace identifier.
 * @returns Query key tuple for share links.
 */
export const shareLinksQueryKey = (workspaceId: string) => ['workspaces', workspaceId, 'share-links'] as const;

/**
 * Constructs the TanStack query key for resolving a public share link token.
 *
 * @param token - Public share link token.
 * @returns Query key tuple for resolved share link.
 */
export const resolvedShareLinkQueryKey = (token: string) => ['share-link', token] as const;

/**
 * TanStack query options for fetching the list of workspaces accessible to the user,
 * ensuring automatic provisioning via `signInUser` before fetching.
 *
 * @param userId - The active user's ID or null.
 * @returns Configured TanStack query options object.
 */
export function workspacesQueryOptions(userId?: string | null) {
  return queryOptions({
    queryKey: workspacesQueryKey(userId),
    queryFn: async () => {
      if (!userId) return [];
      try {
        await signInUser();
      } catch {
        // provision falls through to listWorkspaces
      }
      return await listWorkspaces();
    },
    staleTime: 30_000,
    enabled: Boolean(userId),
  });
}

/**
 * TanStack query options for fetching details and members of a specific workspace.
 *
 * @param workspaceId - Target workspace identifier.
 * @returns Configured TanStack query options object.
 */
export function workspaceDetailQueryOptions(workspaceId: string) {
  return queryOptions({
    queryKey: workspaceDetailQueryKey(workspaceId),
    queryFn: () => getWorkspace(workspaceId),
    staleTime: 30_000,
    enabled: Boolean(workspaceId),
  });
}

/**
 * TanStack query options for fetching active share links for a workspace.
 *
 * @param workspaceId - Target workspace identifier.
 * @returns Configured TanStack query options object.
 */
export function shareLinksQueryOptions(workspaceId: string) {
  return queryOptions({
    queryKey: shareLinksQueryKey(workspaceId),
    queryFn: () => listShareLinks(workspaceId),
    staleTime: 10_000,
    enabled: Boolean(workspaceId),
  });
}

/**
 * TanStack query options for resolving an unauthenticated public share link.
 *
 * @param token - Public share link token.
 * @returns Configured TanStack query options object.
 */
export function resolveShareLinkQueryOptions(token: string) {
  return queryOptions({
    queryKey: resolvedShareLinkQueryKey(token),
    queryFn: () => resolveShareLink(token),
    staleTime: 60_000,
    enabled: Boolean(token),
  });
}
