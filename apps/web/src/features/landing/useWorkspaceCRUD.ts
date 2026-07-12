'use client';

import { useCallback } from 'react';
import { signOutSession, type AuthUser } from '@/features/auth/session';
import { createWorkspace, deleteWorkspace, updateWorkspace, type Workspace } from '@/features/workspaces/api';

const NEW_WORKSPACE_NAME = '新規ワークスペース';

interface UseWorkspaceCRUDArgs {
  user: AuthUser | null;
  workspaces: Workspace[];
  setWorkspaces: React.Dispatch<React.SetStateAction<Workspace[]>>;
  setPendingRevealNodeId: (id: string | null) => void;
  handleOpenWorkspace: (workspaceId: string, overrideWorkspace?: Workspace) => Promise<void>;
  markWorkspaceVerified: (workspaceId: string) => void;
  resetTree: () => void;
  resetRuntimeState: () => void;
  resetToLoggedOutDefaults: (previousUser: AuthUser | null) => void;
}

// useWorkspaceCRUD bundles the workspace-level mutations that the landing
// page exposes through the paper map: rename (manual / auto), create, and
// logout. The hook keeps the orchestrator wiring small and the rename /
// create paths consistent.
export function useWorkspaceCRUD({
  user,
  workspaces,
  setWorkspaces,
  setPendingRevealNodeId,
  handleOpenWorkspace,
  markWorkspaceVerified,
  resetTree,
  resetRuntimeState,
  resetToLoggedOutDefaults,
}: UseWorkspaceCRUDArgs) {
  const applyWorkspaceUpdate = useCallback((workspace: Workspace) => {
    setWorkspaces((prev) => prev.map((candidate) => (
      candidate.workspaceId === workspace.workspaceId ? workspace : candidate
    )));
  }, [setWorkspaces]);

  const handleRenameWorkspace = useCallback(async (workspaceId: string, name: string) => {
    const updated = await updateWorkspace(workspaceId, name);
    applyWorkspaceUpdate(updated);
    return updated;
  }, [applyWorkspaceUpdate]);

  const handleAutoNameWorkspace = useCallback(async (workspaceId: string, suggestedName: string) => {
    const current = workspaces.find((workspace) => workspace.workspaceId === workspaceId);
    if (!current || current.name !== NEW_WORKSPACE_NAME) return;
    const trimmed = suggestedName.trim();
    if (!trimmed || trimmed === current.name) return;
    await handleRenameWorkspace(workspaceId, trimmed.slice(0, 64));
  }, [handleRenameWorkspace, workspaces]);

  const handleCreateWorkspace = useCallback(async (name: string) => {
    const ws = await createWorkspace(name);
    markWorkspaceVerified(ws.workspaceId);
    setWorkspaces((prev) => [ws, ...prev]);
    void handleOpenWorkspace(ws.workspaceId, ws);
    setPendingRevealNodeId(ws.workspaceId);
    return ws;
  }, [handleOpenWorkspace, markWorkspaceVerified, setPendingRevealNodeId, setWorkspaces]);

  const handleDeleteWorkspace = useCallback(async (workspaceId: string) => {
    await deleteWorkspace(workspaceId);
    setWorkspaces((prev) => prev.filter((workspace) => workspace.workspaceId !== workspaceId));
    setPendingRevealNodeId(null);
  }, [setPendingRevealNodeId, setWorkspaces]);

  const handleLogout = useCallback(async () => {
    const previousUser = user;
    await signOutSession();
    resetRuntimeState();
    resetTree();
    resetToLoggedOutDefaults(previousUser);
  }, [resetRuntimeState, resetTree, resetToLoggedOutDefaults, user]);

  return {
    handleRenameWorkspace,
    handleAutoNameWorkspace,
    handleCreateWorkspace,
    handleDeleteWorkspace,
    handleLogout,
  };
}
