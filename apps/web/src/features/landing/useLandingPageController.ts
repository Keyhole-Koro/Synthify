'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { create } from '@bufbuild/protobuf';
import type { Paper } from '@keyhole-koro/paper-in-paper';
import { collection, getDocs, limit, orderBy, query } from 'firebase/firestore';
import { WorkspaceSchema } from '@/gen/proto/synthify/app/v1/workspace_pb';
import { useAuthState } from '@/features/auth/useAuthState';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';
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
      injectMockDocumentRoot: (
        workspaceId: string,
        args?: { itemId?: string; title?: string; description?: string },
      ) => Promise<unknown>;
      createDebugCompletedWorkspace: (
        args?: { workspaceName?: string; paperTitle?: string; paperDescription?: string },
      ) => Promise<unknown>;
      createMockWorkspace: (
        args?: {
          workspaceName?: string;
          documentCount?: number;
          nodesPerDocument?: number;
          documentTitles?: string[];
        },
      ) => unknown;
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
    verifiedWorkspaceIds,
    markWorkspaceVerified,
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
    useCallback((workspaceId: string) => verifiedWorkspaceIds.has(workspaceId), [verifiedWorkspaceIds]),
    loading,
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
  const { handleCreateWorkspace } = crud;

  const handleRootUpload = useRootUpload({
    setWorkspaces,
    setPendingRevealNodeId,
    markWorkspaceVerified,
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
        // Firestore is unavailable for frontend-only mock workspaces; don't let
        // it abort the (more useful) paperMap / projection report.
        let latestJobs: FirestoreJobStatus[] = [];
        try {
          const latestJobsSnap = await getDocs(query(
            collection(db, 'workspaces', workspaceId, 'jobs'),
            orderBy('updatedAt', 'desc'),
            limit(5),
          ));
          latestJobs = latestJobsSnap.docs.map((doc) => doc.data() as FirestoreJobStatus);
        } catch (err) {
          console.warn('[synthify-debug] latestJobs unavailable', err);
        }
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
            paperMapSize: paperMap.size,
            documentRootPaperPresence,
          },
          expansion: {
            workspaceOpenChildIds: expansionMap.get(workspaceId)?.openChildIds ?? [],
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
      injectMockDocumentRoot: async (
        workspaceId: string,
        args: { itemId?: string; title?: string; description?: string } = {},
      ) => {
        const injected = await tree.injectMockDocumentRoot(workspaceId, args);
        const dump = await debugApi.dumpWorkspace(workspaceId);
        return { injected, dump };
      },
      createDebugCompletedWorkspace: async (
        args: { workspaceName?: string; paperTitle?: string; paperDescription?: string } = {},
      ) => {
        const workspace = await handleCreateWorkspace(args.workspaceName?.trim() || 'Debug completed workspace');
        await tree.handleOpenWorkspace(workspace.workspaceId, workspace);
        const injected = await tree.injectMockDocumentRoot(workspace.workspaceId, {
          title: args.paperTitle?.trim() || 'Debug completed paper',
          description: args.paperDescription?.trim() || 'Created by __synthifyDebug.createDebugCompletedWorkspace',
        });
        setPendingRevealNodeId(workspace.workspaceId);
        const dump = await debugApi.dumpWorkspace(workspace.workspaceId);
        return { workspace, injected, dump };
      },
      // createMockWorkspace builds a complete workspace (ws + N documents, each
      // with M nodes) entirely on the frontend — no backend API, no auth, no
      // emulators. Lets you preview WorkspacePaper UI states from the console.
      createMockWorkspace: (
        args: {
          workspaceName?: string;
          documentCount?: number;
          nodesPerDocument?: number;
          documentTitles?: string[];
        } = {},
      ) => {
        const workspaceId = `debug_ws_${Date.now().toString(36)}`;
        const workspace = create(WorkspaceSchema, {
          workspaceId,
          name: args.workspaceName?.trim() || 'Mock workspace',
          ownerId: user?.id ?? 'debug-user',
        });
        setWorkspaces((prev) => [...prev, workspace]);
        markWorkspaceVerified(workspaceId);
        const result = tree.injectMockWorkspaceTree(workspace, {
          documentCount: args.documentCount,
          nodesPerDocument: args.nodesPerDocument,
          documentTitles: args.documentTitles,
        });
        setPendingRevealNodeId(workspaceId);
        console.info('[synthify-debug] mock workspace created [SYNC-MARKER-A1]', { workspace, result });
        return { workspace, result };
      },
    };

    window.__synthifyDebug = debugApi;
    return () => {
      if (window.__synthifyDebug === debugApi) {
        delete window.__synthifyDebug;
      }
    };
  }, [handleCreateWorkspace, paperMap, tree, workspacePaperGroups, setWorkspaces, user, expansionMap]);

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
