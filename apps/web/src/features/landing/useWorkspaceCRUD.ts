import { useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { signOutSession, type AuthUser } from '@/features/auth/session';
import type { Workspace } from '@/features/workspaces/api';
import {
  useCreateWorkspaceMutation,
  useUpdateWorkspaceMutation,
  useDeleteWorkspaceMutation,
} from '@/features/workspaces/mutations';

const NEW_WORKSPACE_NAME = '新規ワークスペース';

interface UseWorkspaceCRUDArgs {
  user: AuthUser | null;
  workspaces: Workspace[];
  setPendingRevealNodeId: (id: string | null) => void;
  handleOpenWorkspace: (workspaceId: string, overrideWorkspace?: Workspace) => Promise<void>;
  markWorkspaceVerified: (workspaceId: string) => void;
  resetTree: () => void;
  resetRuntimeState: () => void;
  resetToLoggedOutDefaults: (previousUser: AuthUser | null) => void;
}

/**
 * Bundles workspace mutations (create, rename, auto-naming, delete, logout)
 * used across the paper canvas landing interface, keeping TanStack Query cache synchronized.
 *
 * @param props - Authenticated user, workspace collection, cache/UI callbacks, and tree reset hooks.
 * @returns Stable handlers for rename, auto-name, create, delete, and logout operations.
 */
export function useWorkspaceCRUD({
  user,
  workspaces,
  setPendingRevealNodeId,
  handleOpenWorkspace,
  markWorkspaceVerified,
  resetTree,
  resetRuntimeState,
  resetToLoggedOutDefaults,
}: UseWorkspaceCRUDArgs) {
  const queryClient = useQueryClient();
  const createMutation = useCreateWorkspaceMutation();
  const updateMutation = useUpdateWorkspaceMutation();
  const deleteMutation = useDeleteWorkspaceMutation();

  const handleRenameWorkspace = useCallback(async (workspaceId: string, name: string) => {
    return await updateMutation.mutateAsync({ workspaceId, name });
  }, [updateMutation]);

  const handleAutoNameWorkspace = useCallback(async (workspaceId: string, suggestedName: string) => {
    const current = workspaces.find((workspace) => workspace.workspaceId === workspaceId);
    if (!current || current.name !== NEW_WORKSPACE_NAME) return;
    const trimmed = suggestedName.trim();
    if (!trimmed || trimmed === current.name) return;
    await handleRenameWorkspace(workspaceId, trimmed.slice(0, 64));
  }, [handleRenameWorkspace, workspaces]);

  const handleCreateWorkspace = useCallback(async (name: string) => {
    const ws = await createMutation.mutateAsync(name);
    markWorkspaceVerified(ws.workspaceId);
    void handleOpenWorkspace(ws.workspaceId, ws);
    setPendingRevealNodeId(ws.workspaceId);
    return ws;
  }, [createMutation, handleOpenWorkspace, markWorkspaceVerified, setPendingRevealNodeId]);

  const handleDeleteWorkspace = useCallback(async (workspaceId: string) => {
    await deleteMutation.mutateAsync(workspaceId);
    setPendingRevealNodeId(null);
  }, [deleteMutation, setPendingRevealNodeId]);

  const handleLogout = useCallback(async () => {
    const previousUser = user;
    await signOutSession();
    queryClient.removeQueries({ queryKey: ['workspaces'] });
    resetRuntimeState();
    resetTree();
    resetToLoggedOutDefaults(previousUser);
  }, [queryClient, resetRuntimeState, resetTree, resetToLoggedOutDefaults, user]);

  return {
    handleRenameWorkspace,
    handleAutoNameWorkspace,
    handleCreateWorkspace,
    handleDeleteWorkspace,
    handleLogout,
  };
}
