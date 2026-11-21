
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
import { useLandingDebugApi } from './useLandingDebugApi';

/**
 * Orchestrates the full landing page state: combines authentication, paper canvas persistence,
 * workspace tree hierarchical loading, drag-and-drop file upload, and workspace CRUD actions.
 *
 * @returns Aggregated landing-page state, derived paper maps, loading/error flags, and stable
 *   handlers consumed by LandingPageView. All returned callbacks preserve the current auth/cache
 *   boundary and clear user-scoped state on logout.
 */
export function useLandingPageController() {
  const {
    user,
    loading,
    workspaces,
    workspaceError,
    authError,
    verifiedWorkspaceIds,
    markWorkspaceVerified,
    handleGoogleSubmit,
    retryWorkspaces,
  } = useAuthState();
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
  const deleteRef = useRef<(workspaceId: string) => Promise<void>>(async () => {});

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
    useCallback((workspaceId: string) => deleteRef.current(workspaceId), []),
    runtimeState.getRuntimeState,
    runtimeState.clearRuntimeState,
    useCallback((workspaceId: string) => verifiedWorkspaceIds.has(workspaceId), [verifiedWorkspaceIds]),
    loading,
  );

  useEffect(() => {
    rebuildWorkspacePaperRef.current = tree.rebuildWorkspacePaper;
  }, [tree.rebuildWorkspacePaper]);

  const crud = useWorkspaceCRUD({
    user,
    workspaces,
    setPendingRevealNodeId,
    handleOpenWorkspace: tree.handleOpenWorkspace,
    markWorkspaceVerified,
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
  useEffect(() => {
    deleteRef.current = crud.handleDeleteWorkspace;
  }, [crud.handleDeleteWorkspace]);
  const { handleCreateWorkspace } = crud;

  const handleRootUpload = useRootUpload({
    setPendingRevealNodeId,
    markWorkspaceVerified,
    handleOpenWorkspace: tree.handleOpenWorkspace,
    uploadWorkspaceFile: tree.uploadWorkspaceFile,
    injectRuntimeState: runtimeState.injectRuntimeState,
  });

  const { isFullscreen } = useHomeCanvasViewState(expansionMap);

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

  useLandingDebugApi({
    user,
    paperMap,
    expansionMap,
    workspacePaperGroups,
    tree,
    handleCreateWorkspace,
    markWorkspaceVerified,
    setPendingRevealNodeId,
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
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    clearPendingRevealNodeId,
  };
}

/**
 * Controller return type encapsulating all landing page state and action handlers.
 */
export type LandingPageController = ReturnType<typeof useLandingPageController>;
