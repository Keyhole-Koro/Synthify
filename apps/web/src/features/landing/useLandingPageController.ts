'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { Paper } from '@keyhole-koro/paper-in-paper';
import { useAuthState } from '@/features/auth/useAuthState';
import { useWorkspaceTree } from '@/features/workspaces/useWorkspaceTree';
import { useHomeCanvasViewState } from '@/features/paperMap/hooks/useHomeCanvasViewState';
import { useLandingPaperMap } from '@/features/paperMap/hooks/useLandingPaperMap';
import { usePersistentPaperOpenState } from '@/features/paperMap/hooks/usePersistentPaperOpenState';
import { useViewportSize } from '@/features/landing/useViewportSize';
import { useWorkspaceRuntimeState } from './useWorkspaceRuntimeState';
import { useWorkspaceCRUD } from './useWorkspaceCRUD';
import { useRootUpload } from './useRootUpload';

// useLandingPageController is the orchestrator for the landing page. It
// wires the auth state, paper map persistence, the workspace tree, and the
// per-workspace CRUD + root upload flows together. All non-trivial logic
// lives in the specialized hooks under landing/ and workspaces/tree/.
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
  const [pendingRevealNodeId, setPendingRevealNodeId] = useState<string | null>(null);
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

  const clearPendingRevealNodeId = useCallback(() => {
    setPendingRevealNodeId(null);
  }, []);

  // The runtime-state hook needs rebuildWorkspacePaper, which is produced by
  // useWorkspaceTree. They reference each other (the tree's
  // onWorkspaceRuntimeStateComplete clears the runtime state, and the
  // runtime state's injectRuntimeState calls rebuildWorkspacePaper). Break
  // the cycle with a ref that points to whichever value is current.
  const rebuildWorkspacePaperRef = useRef<(workspaceId: string) => void>(() => {});
  const runtimeState = useWorkspaceRuntimeState({
    rebuildWorkspacePaper: useCallback((workspaceId: string) => {
      rebuildWorkspacePaperRef.current(workspaceId);
     
    }, []),
  });

  // The CRUD hook also depends on rename, which is what we want to feed
  // into useWorkspaceTree. Construct rename callbacks early and pass the
  // same closures everywhere.
  const renameRef = useRef<(workspaceId: string, name: string) => Promise<unknown>>(async () => undefined);
  const autoNameRef = useRef<(workspaceId: string, suggestedName: string) => Promise<void>>(async () => {});

  const tree = useWorkspaceTree(
    getWorkspaceName,
    expansionMap,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    setWorkspacePapers,
    clearWorkspacePapers,
    workspaces,
    useCallback((workspaceId: string, name: string) => renameRef.current(workspaceId, name) as Promise<import('@/features/workspaces/api').Workspace>, []),
    useCallback((workspaceId: string, suggestedName: string) => autoNameRef.current(workspaceId, suggestedName), []),
    runtimeState.getRuntimeState,
    runtimeState.clearRuntimeState,
  );

  useEffect(() => {
    rebuildWorkspacePaperRef.current = tree.rebuildWorkspacePaper;
  }, [tree.rebuildWorkspacePaper]);

  const crud = useWorkspaceCRUD({
    user,
    workspaces,
    setWorkspaces,
    setPendingRevealNodeId,
    handleOpenWorkspace: tree.handleOpenWorkspace,
    resetTree: tree.resetTree,
    resetRuntimeState: runtimeState.resetAll,
    resetToLoggedOutDefaults,
  });

  useEffect(() => {
    renameRef.current = crud.handleRenameWorkspace;
  }, [crud.handleRenameWorkspace]);
  useEffect(() => {
    autoNameRef.current = crud.handleAutoNameWorkspace;
  }, [crud.handleAutoNameWorkspace]);

  const handleRootUpload = useRootUpload({
    setWorkspaces,
    setPendingRevealNodeId,
    handleOpenWorkspace: tree.handleOpenWorkspace,
    uploadWorkspaceFile: tree.uploadWorkspaceFile,
    injectRuntimeState: runtimeState.injectRuntimeState,
  });

  const { isFullscreen } = useHomeCanvasViewState(expansionMap, canvasFullscreen);

  const { paperMap } = useLandingPaperMap({
    user,
    loading,
    workspaces,
    workspaceError,
    authError,
    workspacePaperGroups,
    handleGoogleSubmit,
    handleLogout: crud.handleLogout,
    handleCreateWorkspace: crud.handleCreateWorkspace,
    handleRootUpload,
    handleOpenWorkspace: tree.handleOpenWorkspace,
    buildWsPaper: tree.buildWsPaper,
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
    pendingRevealNodeId,
    setCanvasFullscreen,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    clearPendingRevealNodeId,
  };
}

export type LandingPageController = ReturnType<typeof useLandingPageController>;
