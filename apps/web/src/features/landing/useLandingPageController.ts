'use client';

import { useCallback, useState } from 'react';
import type { Paper } from '@keyhole-koro/paper-in-paper';
import { useAuthState } from '@/features/auth/useAuthState';
import { signOutSession } from '@/features/auth/session';
import { createWorkspace, updateWorkspace, type Workspace } from '@/features/workspaces/api';
import { useWorkspaceTree } from '@/features/workspaces/useWorkspaceTree';
import { useHomeCanvasViewState } from '@/features/paperMap/hooks/useHomeCanvasViewState';
import { useLandingPaperMap } from '@/features/paperMap/hooks/useLandingPaperMap';
import { usePersistentPaperOpenState } from '@/features/paperMap/hooks/usePersistentPaperOpenState';
import { useViewportSize } from '@/features/landing/useViewportSize';

const NEW_WORKSPACE_NAME = '新規ワークスペース';

export function useLandingPageController() {
  const {
    user,
    loading,
    workspaces,
    workspaceError,
    authError,
    setWorkspaces,
    handleGoogleSubmit,
    retryWorkspaces,
  } = useAuthState();
  const [canvasFullscreen, setCanvasFullscreen] = useState(false);
  const [workspacePaperGroups, setWorkspacePaperGroups] = useState<Map<string, Paper[]>>(new Map());
  const { hasMounted, winSize } = useViewportSize();

  const {
    canvasKey,
    defaultOpenState,
    hasDefaultOpenState,
    expansionMap,
    focusedNodeId,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    resetToLoggedOutDefaults,
  } = usePersistentPaperOpenState({ user, loading, workspaces });

  const getWorkspaceName = useCallback(
    (id: string) => workspaces.find((w) => w.workspaceId === id)?.name ?? id,
    [workspaces],
  );

  const setWorkspacePapers = useCallback((workspaceId: string, papers: Paper[]) => {
    setWorkspacePaperGroups((prev) => {
      const current = prev.get(workspaceId);
      if (current === papers) return prev;
      const next = new Map(prev);
      next.set(workspaceId, papers);
      return next;
    });
  }, []);

  const clearWorkspacePapers = useCallback(() => {
    setWorkspacePaperGroups(new Map());
  }, []);

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

  const { handleOpenWorkspace, refreshWorkspaceTree, resetTree, buildWsPaper, uploadWorkspaceFile } = useWorkspaceTree(
    getWorkspaceName,
    expansionMap,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    setWorkspacePapers,
    clearWorkspacePapers,
    workspaces,
    handleRenameWorkspace,
    handleAutoNameWorkspace,
  );

  const { isFullscreen } = useHomeCanvasViewState(expansionMap, canvasFullscreen);

  const handleLogout = useCallback(async () => {
    const previousUser = user;
    await signOutSession();
    resetTree();
    resetToLoggedOutDefaults(previousUser);
  }, [resetToLoggedOutDefaults, resetTree, user]);

  const handleCreateWorkspace = useCallback(async (name: string) => {
    const ws = await createWorkspace(name);
    setWorkspaces((prev) => [ws, ...prev]);
    void handleOpenWorkspace(ws.workspaceId, ws);
  }, [handleOpenWorkspace, setWorkspaces]);

  const handleRootUpload = useCallback(async (file: File) => {
    const ws = await createWorkspace(NEW_WORKSPACE_NAME);
    setWorkspaces((prev) => [ws, ...prev]);
    await handleOpenWorkspace(ws.workspaceId, ws);
    const uploaded = await uploadWorkspaceFile(ws.workspaceId, file);
    return {
      workspaceId: ws.workspaceId,
      workspaceName: ws.name,
      jobId: uploaded.jobId,
      documentId: uploaded.documentId,
    };
  }, [handleOpenWorkspace, setWorkspaces, uploadWorkspaceFile]);

  const handleRootUploadComplete = useCallback(async (workspaceId: string) => {
    await refreshWorkspaceTree(workspaceId, { revealNewDocumentRoots: true });
  }, [refreshWorkspaceTree]);

  const { paperMap } = useLandingPaperMap({
    user,
    loading,
    workspaces,
    workspaceError,
    authError,
    workspacePaperGroups,
    handleGoogleSubmit,
    handleLogout,
    handleCreateWorkspace,
    handleRootUpload,
    handleRootUploadComplete,
    handleSuggestedWorkspaceName: handleAutoNameWorkspace,
    handleOpenWorkspace,
    buildWsPaper,
    onRetryWorkspaces: retryWorkspaces,
  });

  return {
    isReady: hasMounted && hasDefaultOpenState,
    isFullscreen,
    winSize,
    paperMap,
    canvasKey,
    defaultOpenState,
    expansionMap,
    focusedNodeId,
    setCanvasFullscreen,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
  };
}

export type LandingPageController = ReturnType<typeof useLandingPageController>;
