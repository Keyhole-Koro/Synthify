import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createShareLink, revokeShareLink, type WorkspaceRole } from '@/features/workspaces/api';
import { shareLinksQueryKey } from '@/features/workspaces/queries';

/**
 * Mutation hook for generating a public or role-based workspace sharing link.
 *
 * @param workspaceId - Workspace identifier.
 * @returns Mutation object accepting `{ role, expiresAt }`; success invalidates share-link data.
 */
export function useCreateShareLinkMutation(workspaceId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ role, expiresAt = '' }: { role: WorkspaceRole; expiresAt?: string }) =>
      createShareLink(workspaceId, role, expiresAt),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: shareLinksQueryKey(workspaceId) });
    },
  });
}

/**
 * Mutation hook for revoking an active workspace share link token.
 *
 * @param workspaceId - Workspace identifier.
 * @returns Mutation object accepting a token; success invalidates share-link data.
 */
export function useRevokeShareLinkMutation(workspaceId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (token: string) => revokeShareLink(workspaceId, token),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: shareLinksQueryKey(workspaceId) });
    },
  });
}
