'use client';

import { useCallback, useState } from 'react';
import type { ExpansionMap, Paper } from '@keyhole-koro/paper-in-paper';
import { useAuthState } from '@/features/auth/useAuthState';
import { signOutSession } from '@/features/auth/session';
import { createWorkspace } from '@/features/workspaces/api';
import { useWorkspaceTree } from '@/features/workspaces/useWorkspaceTree';
import { useHomeCanvasViewState } from '@/features/paperMap/hooks/useHomeCanvasViewState';
import { useLandingPaperMap } from '@/features/paperMap/hooks/useLandingPaperMap';
import { usePersistentPaperOpenState } from '@/features/paperMap/hooks/usePersistentPaperOpenState';
import type { AuthMode } from '@/features/auth/AuthPaper';
import { useViewportSize } from '@/features/landing/useViewportSize';

export function useLandingPageController() {
  const {
    user,
    loading,
    workspaces,
    workspaceError,
    setWorkspaces,
    handleGoogleSubmit,
    handleEmailSubmit,
  } = useAuthState();
  const [authMode, setAuthMode] = useState<AuthMode>('login');
  const [canvasFullscreen, setCanvasFullscreen] = useState(false);
  const [workspacePaperGroups, setWorkspacePaperGroups] = useState<Map<string, Paper[]>>(new Map());
  const { hasMounted, winSize } = useViewportSize();

  const {
    canvasKey,
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

  const { handleOpenWorkspace, resetTree, buildWsPaper } = useWorkspaceTree(
    getWorkspaceName,
    expansionMap,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    setWorkspacePapers,
    clearWorkspacePapers,
    workspaces,
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
    setWorkspaces((prev) => [...prev, ws]);
    void handleOpenWorkspace(ws.workspaceId);
  }, [handleOpenWorkspace, setWorkspaces]);

  const { paperMap } = useLandingPaperMap({
    user,
    loading,
    workspaces,
    workspaceError,
    authMode,
    workspacePaperGroups,
    setAuthMode,
    handleEmailSubmit,
    handleGoogleSubmit,
    handleLogout,
    handleCreateWorkspace,
    handleOpenWorkspace,
    buildWsPaper,
  });

  const handleControlledExpansionMapChange = useCallback((map: ExpansionMap) => {
    handleExpansionMapChange(map);
  }, [handleExpansionMapChange]);

  return {
    isReady: hasMounted && !loading,
    isFullscreen,
    winSize,
    paperMap,
    canvasKey,
    expansionMap,
    focusedNodeId,
    setCanvasFullscreen,
    handleExpansionMapChange: handleControlledExpansionMapChange,
    handleFocusedNodeIdChange,
  };
}

export type LandingPageController = ReturnType<typeof useLandingPageController>;
