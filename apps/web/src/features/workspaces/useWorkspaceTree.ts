'use client';

import { useCallback, useEffect, useMemo, useRef } from 'react';
import type { ExpansionMap, Paper } from '@keyhole-koro/paper-in-paper';
import { projectWorkspacePapers } from '@/features/workspaces/useWorkspaceProjection';
import { ROOT_ID, WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import { type Workspace } from '@/features/workspaces/api';
import type { WorkspacePaperRuntimeState } from '@/features/workspaces/paper/WorkspacePaper';
import { createWorkspaceTreeCache } from './tree/workspaceTreeCache';
import { createWorkspaceTreeCommands } from './tree/workspaceTreeCommands';
import type { InjectMockDocumentRootArgs, InjectMockWorkspaceTreeArgs, MergeDocumentRootResult, TreeStore } from './tree/workspaceTreeTypes';
import { useWorkspaceUpload } from './upload/useWorkspaceUpload';
import { useExpansionWatcher } from './tree/useExpansionWatcher';
import { createWorkspacePaperFactory } from './paper/workspacePaperFactory';
export type { WorkspacePaperRuntimeState } from '@/features/workspaces/paper/WorkspacePaper';

// useWorkspaceTree is the orchestrator. It owns no data of its own — the
// tree cache lives in workspaceTreeCache, API calls live in
// workspaceTreeCommands, the paper factory in workspacePaperFactory, and the
// expansion side effects in useExpansionWatcher. This hook wires those
// boundaries together.
export function useWorkspaceTree(
  getWorkspaceName: (id: string) => string,
  expansionMap: ExpansionMap,
  onExpansionMapChange: (expansionMap: ExpansionMap) => void,
  onFocusedNodeIdChange: (nodeId: string | null) => void,
  setWorkspacePapers: (workspaceId: string, papers: Paper[]) => void,
  clearWorkspacePapers: () => void,
  workspaces: Workspace[],
  onRenameWorkspace: (workspaceId: string, name: string) => Promise<Workspace>,
  onSuggestedWorkspaceName: (workspaceId: string, suggestedName: string) => Promise<void>,
  getWorkspacePaperRuntimeState: (workspaceId: string) => WorkspacePaperRuntimeState = () => ({}),
  onWorkspaceRuntimeStateComplete: (workspaceId: string) => void = () => {},
  canFetchWorkspaceTree: (workspaceId: string) => boolean = () => true,
  loading = false,
) {
  const expansionMapRef = useRef<ExpansionMap>(expansionMap);
  useEffect(() => {
    expansionMapRef.current = expansionMap;
  }, [expansionMap]);

  const treeCache = useMemo(() => createWorkspaceTreeCache(), []);
  const treeCommands = useMemo(() => createWorkspaceTreeCommands(treeCache), [treeCache]);
  useEffect(() => {
    if (workspaces.length === 0) return;
    treeCache.pruneNewlyCreated(workspaces);
  }, [treeCache, workspaces]);

  const store = useMemo<TreeStore>(() => ({
    getRootItemId: treeCache.getRootItemId,
    getDocumentRootIds: treeCache.getDocumentRootIds,
    getTreeItems: treeCache.getTreeItems,
    getItemWorkspaceId: treeCache.getItemWorkspaceId,
    hasInitialized: treeCache.hasInitialized,
    isLoaded: treeCache.isLoaded,
    isLoading: treeCache.isLoading,
    hasChildren: treeCache.hasChildren,
    isFullyLoaded: treeCache.isFullyLoaded,
    getNewlyCreated: treeCache.getNewlyCreated,
    listNewlyCreatedIds: treeCache.listNewlyCreatedIds,
    markInitialized: treeCache.markInitialized,
    rememberNewlyCreated: treeCache.rememberNewlyCreated,
    refreshWorkspaceTree: treeCommands.refreshWorkspaceTree,
    loadSubtree: treeCommands.loadSubtree,
    mergeDocumentRoot: treeCommands.mergeDocumentRoot,
    injectMockDocumentRoot: treeCache.injectMockDocumentRoot,
    injectMockWorkspaceTree: treeCache.injectMockWorkspaceTree,
    debugSnapshot: treeCache.debugSnapshot,
    reset: treeCache.reset,
  }), [treeCache, treeCommands]);
  const handleUploadWorkspaceFile = useWorkspaceUpload();

  const canReadWorkspaceTree = useCallback((workspaceId: string) => (
    canFetchWorkspaceTree(workspaceId) || store.getNewlyCreated(workspaceId) !== undefined
  ), [canFetchWorkspaceTree, store]);

  // openChild / setOpenChildren / updateWorkspaceExpansion are pure
  // ExpansionMap manipulations — kept inline because they are the only
  // place the orchestrator touches the map directly.
  const setOpenChildren = (parentId: string, childIds: string[], base: ExpansionMap): ExpansionMap => {
    const next = new Map(base);
    next.set(parentId, { openChildIds: childIds });
    return next;
  };

  const openChild = (parentId: string, childId: string, base: ExpansionMap): ExpansionMap => {
    const current = base.get(parentId)?.openChildIds ?? [];
    if (current.includes(childId)) return base;
    return setOpenChildren(parentId, [...current, childId], base);
  };

  const updateWorkspaceExpansion = useCallback((
    workspaceId: string,
    newDocumentRootIds: string[] = [],
    revealNewDocumentRoots = false,
  ) => {
    let map = expansionMapRef.current;
    map = openChild(ROOT_ID, WORKSPACES_ID, map);
    map = openChild(WORKSPACES_ID, workspaceId, map);

    const allTreeItemIds = new Set(store.getTreeItems(workspaceId).keys());
    const currentWorkspaceOpenIds = (map.get(workspaceId)?.openChildIds ?? []).filter((id) => allTreeItemIds.has(id));
    const openChildIds = revealNewDocumentRoots
      ? Array.from(new Set([...currentWorkspaceOpenIds, ...newDocumentRootIds]))
      : currentWorkspaceOpenIds;

    map = setOpenChildren(workspaceId, openChildIds, map);
    onExpansionMapChange(map);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onExpansionMapChange, store]);

  // mergeDocumentRootIntoTree fires after a PROCESS_DOCUMENT job completes.
  // Defined further down (it depends on runProjectWorkspacePapers, which in
  // turn depends on buildWsPaper). The factory captures it through this ref
  // so the order of declarations works out.
  const mergeDocumentRootIntoTreeRef = useRef<(workspaceId: string, createdDocumentRootItemId: string) => Promise<void>>(
    async () => {},
  );

  // getWorkspace reads the live `workspaces` prop directly (not a ref).
  // handleOpenWorkspace is invoked from the [workspaces]-effect in
  // useExpansionWatcher right after the list loads on reload. A
  // ref-backed workspacesRef is only updated by its own useEffect, which
  // has not run yet at that point, so reading it there yielded the stale
  // (empty) list — getWorkspace returned undefined and buildWsPaper baked a
  // "ワークスペースが見つかりません" paper into workspacePaperGroups, where it
  // stuck (useLandingPaperMap only rebuilds empty groups). The prop is
  // always current during render, so prefer it.
  const getWorkspace = useCallback(
    (id: string) =>
      workspaces.find((w) => w.workspaceId === id) ?? store.getNewlyCreated(id),
  // store getter is stable.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  [workspaces],
  );

  const onProcessingComplete = useCallback(async (workspaceId: string, createdDocumentRootItemId: string) => {
    onWorkspaceRuntimeStateComplete(workspaceId);
    await mergeDocumentRootIntoTreeRef.current(workspaceId, createdDocumentRootItemId);
  }, [onWorkspaceRuntimeStateComplete]);

  // The factory reads getWorkspace / onProcessingComplete at render time
  // only as closures — the actual ref dereference happens later, inside the
  // returned buildWsPaper invocation. react-hooks/refs cannot tell the
  // difference, so suppress it locally.
  const isLoading = useCallback(() => loading, [loading]);

  const buildWsPaper = useMemo(
    // eslint-disable-next-line react-hooks/refs
    () => createWorkspacePaperFactory({
      getWorkspace,
      isLoading,
      getWorkspaceName,
      getRuntimeState: getWorkspacePaperRuntimeState,
      onUploadFile: handleUploadWorkspaceFile,
      onRenameWorkspace,
      onSuggestedWorkspaceName,
      onProcessingComplete,
    }),
    [getWorkspace, isLoading, getWorkspaceName, getWorkspacePaperRuntimeState, handleUploadWorkspaceFile,
      onRenameWorkspace, onSuggestedWorkspaceName, onProcessingComplete],
  );

  const runProjectWorkspacePapers = useCallback((workspaceId: string, workspaceRootItemId: string): Paper[] => {
    store.markInitialized(workspaceId);
    return projectWorkspacePapers(
      workspaceId,
      workspaceRootItemId,
      store.getTreeItems(workspaceId),
      store.getDocumentRootIds(workspaceId),
      buildWsPaper,
    );
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildWsPaper]);

  const refreshWorkspaceTree = useCallback(async (
    workspaceId: string,
    opts: { revealNewDocumentRoots?: boolean } = {},
  ) => {
    if (!canReadWorkspaceTree(workspaceId)) {
      return;
    }
    const { rootItemId, newDocumentRootIds } = await store.refreshWorkspaceTree(workspaceId);
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, rootItemId));
    updateWorkspaceExpansion(workspaceId, newDocumentRootIds, opts.revealNewDocumentRoots === true);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canReadWorkspaceTree, runProjectWorkspacePapers, setWorkspacePapers, updateWorkspaceExpansion]);

  // mergeDocumentRootIntoTree handles a PROCESS_DOCUMENT completion: store
  // merges the new subtree, orchestrator re-projects and reveals the new
  // document_root in the expansion map.
  const mergeDocumentRootIntoTree = useCallback(async (
    workspaceId: string,
    createdDocumentRootItemId: string,
  ) => {
    if (!canReadWorkspaceTree(workspaceId)) return;
    if (!store.getRootItemId(workspaceId)) {
      await refreshWorkspaceTree(workspaceId, { revealNewDocumentRoots: true });
      return;
    }
    const merged = await store.mergeDocumentRoot(workspaceId, createdDocumentRootItemId);
    if (!merged) {
      await refreshWorkspaceTree(workspaceId, { revealNewDocumentRoots: true });
      return;
    }
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, merged.workspaceRootItemId));
    updateWorkspaceExpansion(workspaceId, [createdDocumentRootItemId], true);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canReadWorkspaceTree, refreshWorkspaceTree, runProjectWorkspacePapers, setWorkspacePapers, updateWorkspaceExpansion]);

  useEffect(() => {
    mergeDocumentRootIntoTreeRef.current = mergeDocumentRootIntoTree;
  }, [mergeDocumentRootIntoTree]);

  const projectMergedDocumentRoot = useCallback((
    workspaceId: string,
    documentRootItemId: string,
    merged: MergeDocumentRootResult,
  ) => {
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, merged.workspaceRootItemId));
    updateWorkspaceExpansion(workspaceId, [documentRootItemId], true);
  }, [runProjectWorkspacePapers, setWorkspacePapers, updateWorkspaceExpansion]);

  const injectMockDocumentRoot = useCallback(async (
    workspaceId: string,
    args: InjectMockDocumentRootArgs = {},
  ) => {
    if (!canReadWorkspaceTree(workspaceId)) {
      throw new Error(`workspace ${workspaceId} is not verified for tree access`);
    }
    if (!store.getRootItemId(workspaceId)) {
      await refreshWorkspaceTree(workspaceId, { revealNewDocumentRoots: true });
    }
    const merged = store.injectMockDocumentRoot(workspaceId, args);
    if (!merged) {
      throw new Error(`workspace ${workspaceId} has no loaded workspace_root item`);
    }
    const itemId = merged.items[0]?.item?.id;
    if (!itemId) {
      throw new Error('mock document root injection produced no item');
    }
    projectMergedDocumentRoot(workspaceId, itemId, merged);
    return { documentRootItemId: itemId, title: merged.items[0].item!.title };
  }, [canReadWorkspaceTree, projectMergedDocumentRoot, refreshWorkspaceTree, store]);

  // injectMockWorkspaceTree builds a complete frontend-only tree (workspace_root
  // + N document_root, each with M nodes) and projects it, without any backend
  // call. Used by __synthifyDebug to preview WorkspacePaper UI states offline.
  const injectMockWorkspaceTree = useCallback((
    workspace: Workspace,
    args: InjectMockWorkspaceTreeArgs = {},
  ) => {
    const workspaceId = workspace.workspaceId;
    store.rememberNewlyCreated(workspace);
    const result = store.injectMockWorkspaceTree(workspaceId, args);
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, result.rootItemId));
    
    // Open the workspace itself, and also the first document root if available
    // so the user sees some "knowledge tree" nodes immediately.
    const openIds = result.documentRootIds.length > 0
      ? [result.documentRootIds[0]]
      : [];
    updateWorkspaceExpansion(workspaceId, result.documentRootIds, true);
    if (openIds.length > 0) {
      onExpansionMapChange(openChild(workspaceId, openIds[0], expansionMapRef.current));
    }
    
    return result;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runProjectWorkspacePapers, setWorkspacePapers, updateWorkspaceExpansion, onExpansionMapChange]);

  const rebuildWorkspacePaper = useCallback((workspaceId: string) => {
    const rootItemId = store.getRootItemId(workspaceId);
    if (rootItemId) {
      setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, rootItemId));
    } else {
      setWorkspacePapers(workspaceId, [buildWsPaper(workspaceId, [])]);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildWsPaper, runProjectWorkspacePapers, setWorkspacePapers]);

  // loadSubtreeAndProject fetches a subtree, projects Papers, and then
  // recursively kicks loads for any children the user already has open.
  // The recursion sees itself through loadSubtreeAndProjectRef so React's
  // useCallback doesn't trip over the self-reference.
  const loadSubtreeAndProjectRef = useRef<(workspaceId: string, itemId: string, maxDepth?: number) => Promise<void>>(
    async () => {},
  );

  const loadSubtreeAndProject = useCallback(async (
    workspaceId: string,
    itemId: string,
    maxDepth = 1,
  ) => {
    if (!canReadWorkspaceTree(workspaceId)) return;
    const rootItemId = store.getRootItemId(workspaceId);
    if (!rootItemId) return;
    const items = await store.loadSubtree(workspaceId, itemId, maxDepth);
    if (items.length === 0) return;
    for (const item of items) {
      const id = item.item!.id;
      if (!item.hasChildren) continue;
      const isOpen = (expansionMapRef.current.get(id)?.openChildIds.length ?? 0) > 0;
      if (!isOpen) continue;
      if (store.isLoaded(id) || store.isLoading(id)) continue;
      void loadSubtreeAndProjectRef.current(workspaceId, id, 1);
    }
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, rootItemId));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canReadWorkspaceTree, runProjectWorkspacePapers, setWorkspacePapers]);

  useEffect(() => {
    loadSubtreeAndProjectRef.current = loadSubtreeAndProject;
  }, [loadSubtreeAndProject]);

  const handleOpenWorkspace = useCallback(async (workspaceId: string, overrideWorkspace?: Workspace) => {
    if (overrideWorkspace) {
      store.rememberNewlyCreated(overrideWorkspace);
    }
    const knownRootId = store.getRootItemId(workspaceId);
    if (knownRootId) {
      if (!store.isLoaded(knownRootId)) {
        void loadSubtreeAndProject(workspaceId, knownRootId, 1);
      }
      updateWorkspaceExpansion(workspaceId);
      onFocusedNodeIdChange(workspaceId);
      return;
    }
    store.markInitialized(workspaceId);
    setWorkspacePapers(workspaceId, [buildWsPaper(workspaceId, [])]);
    updateWorkspaceExpansion(workspaceId);
    onFocusedNodeIdChange(workspaceId);
    if (!canReadWorkspaceTree(workspaceId)) {
      return;
    }
    await refreshWorkspaceTree(workspaceId);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildWsPaper, canReadWorkspaceTree, loadSubtreeAndProject, onFocusedNodeIdChange, refreshWorkspaceTree, setWorkspacePapers, updateWorkspaceExpansion]);

  useExpansionWatcher({
    expansionMap,
    workspaces,
    store,
    onLoadSubtree: (workspaceId, itemId) => {
      void loadSubtreeAndProject(workspaceId, itemId, 1);
    },
    onOpenWorkspace: (workspaceId) => {
      void handleOpenWorkspace(workspaceId);
    },
    onRebuildPaper: rebuildWorkspacePaper,
    onExpansionMapChange,
  });

  const resetTree = useCallback(() => {
    store.reset();
    clearWorkspacePapers();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clearWorkspacePapers]);

  return {
    handleOpenWorkspace,
    refreshWorkspaceTree,
    mergeDocumentRootIntoTree,
    injectMockDocumentRoot,
    injectMockWorkspaceTree,
    getDebugSnapshot: store.debugSnapshot,
    resetTree,
    buildWsPaper,
    rebuildWorkspacePaper,
    uploadWorkspaceFile: handleUploadWorkspaceFile,
  };
}
