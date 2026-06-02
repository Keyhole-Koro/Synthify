'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { Paper } from '@keyhole-koro/paper-in-paper';
import { collection, getDocs, limit, orderBy, query } from 'firebase/firestore';
import { useAuthState } from '@/features/auth/useAuthState';
import type { FirestoreJobStatus } from '@/features/jobs/useJobStatus';
import { useWorkspaceTree } from '@/features/workspaces/useWorkspaceTree';
import { useHomeCanvasViewState } from '@/features/paperMap/hooks/useHomeCanvasViewState';
import { useLandingPaperMap } from '@/features/paperMap/hooks/useLandingPaperMap';
import { usePersistentPaperOpenState } from '@/features/paperMap/hooks/usePersistentPaperOpenState';
import { useViewportSize } from '@/features/landing/useViewportSize';
import { useWorkspaceRuntimeState } from './useWorkspaceRuntimeState';
import { useWorkspaceCRUD } from './useWorkspaceCRUD';
import { useRootUpload } from './useRootUpload';
import { db } from '@/lib/firebase';

declare global {
  interface Window {
    __synthifyDebug?: {
      dumpWorkspace: (workspaceId: string) => Promise<unknown>;
      refreshWorkspaceTree: (workspaceId: string) => Promise<void>;
      mergeDocumentRoot: (workspaceId: string, documentRootItemId: string) => Promise<void>;
    };
  }
}

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

  useEffect(() => {
    if (process.env.NODE_ENV === 'production' || typeof window === 'undefined') return;

    const debugApi = {
      dumpWorkspace: async (workspaceId: string) => {
        const latestJobsSnap = await getDocs(query(
          collection(db, 'workspaces', workspaceId, 'jobs'),
          orderBy('updatedAt', 'desc'),
          limit(5),
        ));
        const latestJobs = latestJobsSnap.docs.map((doc) => doc.data() as FirestoreJobStatus);
        const workspacePapers = workspacePaperGroups.get(workspaceId) ?? [];
        const treeSnapshot = tree.getDebugSnapshot(workspaceId);
        const documentRootPaperPresence = treeSnapshot.documentRootIds.map((id) => ({
          id,
          inPaperMap: paperMap.has(id),
          inWorkspacePaperGroup: workspacePapers.some((paper) => paper.id === id),
        }));

        const dump = {
          workspaceId,
          tree: treeSnapshot,
          workspacePaperGroup: {
            paperIds: workspacePapers.map((paper) => paper.id),
            paperCount: workspacePapers.length,
          },
          paperMap: {
            hasWorkspacePaper: paperMap.has(workspaceId),
            documentRootPaperPresence,
          },
          latestJobs,
        };
        console.info('[synthify-debug] workspace dump', dump);
        return dump;
      },
      refreshWorkspaceTree: async (workspaceId: string) => {
        await tree.refreshWorkspaceTree(workspaceId, { revealNewDocumentRoots: true });
      },
      mergeDocumentRoot: async (workspaceId: string, documentRootItemId: string) => {
        await tree.mergeDocumentRootIntoTree(workspaceId, documentRootItemId);
      },
    };

    window.__synthifyDebug = debugApi;
    return () => {
      if (window.__synthifyDebug === debugApi) {
        delete window.__synthifyDebug;
      }
    };
  }, [paperMap, tree, workspacePaperGroups]);

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
