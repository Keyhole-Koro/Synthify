'use client';

import { useCallback, useEffect, useMemo, useRef } from 'react';
import type { ExpansionMap, Paper } from '@keyhole-koro/paper-in-paper';
import { projectWorkspacePapers } from '@/features/workspaces/useWorkspaceProjection';
import { ROOT_ID, WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import { type Workspace } from '@/features/workspaces/api';
import type { SubtreeItem } from '@/features/tree/api';
import type { WorkspacePaperRuntimeState } from '@/features/workspaces/paper/WorkspacePaper';
import { createWorkspaceTreeCache } from './tree/workspaceTreeCache';
import { createWorkspaceTreeCommands } from './tree/workspaceTreeCommands';
import type { InjectMockNodeArgs, InjectMockWorkspaceTreeArgs, TreeStore } from './tree/workspaceTreeTypes';
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
  onDeleteWorkspace: (workspaceId: string) => Promise<void>,
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
    treeCache.pruneNewlyCreated(workspaces.filter((workspace) => treeCache.isFullyLoaded(workspace.workspaceId)));
  }, [treeCache, workspaces]);

  const store = useMemo<TreeStore>(() => ({
    getRootNodeIds: treeCache.getRootNodeIds,
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
    injectMockNode: treeCache.injectMockNode,
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

  // openDescendants opens every paper from `rootIds` down to `openDepth`
  // levels, in one pass over the tree. Used by the mock-tree injection path so
  // a load test can control how much of a large tree is actually rendered.
  const openDescendants = (
    base: ExpansionMap,
    rootIds: string[],
    openDepth: number,
    treeItems: Map<string, SubtreeItem>,
  ): ExpansionMap => {
    const next = new Map(base);
    let frontier = rootIds;
    for (let level = 1; level < openDepth && frontier.length > 0; level++) {
      const nextFrontier: string[] = [];
      for (const id of frontier) {
        const childIds = treeItems.get(id)?.item?.childIds ?? [];
        if (childIds.length === 0) continue;
        next.set(id, { openChildIds: childIds });
        nextFrontier.push(...childIds);
      }
      frontier = nextFrontier;
    }
    return next;
  };

  const updateWorkspaceExpansion = useCallback((
    workspaceId: string,
    newDocumentRootIds: string[] = [],
    revealNewDocumentRoots = false,
  ): ExpansionMap => {
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
    return map;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onExpansionMapChange, store]);

  // refreshWorkspaceTreeRef fires after a PROCESS_DOCUMENT job reports
  // treeChanged. A job may create many root nodes or merge into existing ones,
  // so we refetch the whole workspace tree and re-project rather than grafting
  // a single subtree. Defined further down (it depends on
  // runProjectWorkspacePapers / buildWsPaper); the factory captures it through
  // this ref so the order of declarations works out.
  const refreshWorkspaceTreeRef = useRef<(workspaceId: string) => Promise<void>>(
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

  const onProcessingComplete = useCallback(async (workspaceId: string) => {
    onWorkspaceRuntimeStateComplete(workspaceId);
    await refreshWorkspaceTreeRef.current(workspaceId);
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
      onDeleteWorkspace,
      onProcessingComplete,
    }),
    [getWorkspace, isLoading, getWorkspaceName, getWorkspacePaperRuntimeState, handleUploadWorkspaceFile,
      onRenameWorkspace, onSuggestedWorkspaceName, onDeleteWorkspace, onProcessingComplete],
  );

  const runProjectWorkspacePapers = useCallback((workspaceId: string): Paper[] => {
    store.markInitialized(workspaceId);
    return projectWorkspacePapers(
      workspaceId,
      store.getTreeItems(workspaceId),
      store.getRootNodeIds(workspaceId),
      buildWsPaper,
    );
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildWsPaper]);

  const refreshWorkspaceTree = useCallback(async (
    workspaceId: string,
    opts: { revealNewRootNodes?: boolean } = {},
  ) => {
    if (!canReadWorkspaceTree(workspaceId)) {
      return;
    }
    const { newRootNodeIds } = await store.refreshWorkspaceTree(workspaceId);
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId));
    updateWorkspaceExpansion(workspaceId, newRootNodeIds, opts.revealNewRootNodes === true);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canReadWorkspaceTree, runProjectWorkspacePapers, setWorkspacePapers, updateWorkspaceExpansion]);

  // After a PROCESS_DOCUMENT job reports treeChanged, refetch the whole
  // workspace tree and reveal any newly created root nodes.
  useEffect(() => {
    refreshWorkspaceTreeRef.current = (workspaceId: string) =>
      refreshWorkspaceTree(workspaceId, { revealNewRootNodes: true });
  }, [refreshWorkspaceTree]);

  const injectMockNode = useCallback(async (
    workspaceId: string,
    args: InjectMockNodeArgs = {},
  ) => {
    if (!canReadWorkspaceTree(workspaceId)) {
      throw new Error(`workspace ${workspaceId} is not verified for tree access`);
    }
    const itemId = store.injectMockNode(workspaceId, args);
    if (!itemId) {
      throw new Error('mock node injection produced no item');
    }
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId));
    updateWorkspaceExpansion(workspaceId, [itemId], true);
    const title = store.getTreeItems(workspaceId).get(itemId)?.item?.title ?? '';
    return { rootNodeItemId: itemId, title };
  }, [canReadWorkspaceTree, runProjectWorkspacePapers, setWorkspacePapers, store, updateWorkspaceExpansion]);

  // injectMockWorkspaceTree builds a complete frontend-only tree and projects
  // it, without any backend call. Used by __synthifyDebug to preview
  // WorkspacePaper UI states offline, and by the load-test harness to hand the
  // client an arbitrary number of items.
  //
  // The two phases that scale with item count are timed separately so a slow
  // result can be attributed rather than guessed at. injectMs covers mock
  // generation plus the cache rebuild — in a real load it is GetTree's decode
  // plus that same cache rebuild. projectMs is the paper projection, which is
  // the part that also runs on every subsequent expansion.
  const injectMockWorkspaceTree = useCallback((
    workspace: Workspace,
    args: InjectMockWorkspaceTreeArgs = {},
  ) => {
    const workspaceId = workspace.workspaceId;
    store.rememberNewlyCreated(workspace);

    const injectStartedAt = performance.now();
    const result = store.injectMockWorkspaceTree(workspaceId, args);
    const injectMs = performance.now() - injectStartedAt;

    const projectStartedAt = performance.now();
    const papers = runProjectWorkspacePapers(workspaceId);
    const projectMs = performance.now() - projectStartedAt;
    setWorkspacePapers(workspaceId, papers);

    // Open the workspace, then its root node(s), then as many further levels as
    // openDepth asks for. Both expansion writes are folded into one map so the
    // second does not build on a stale expansionMapRef and drop the first.
    const withWorkspaceOpen = updateWorkspaceExpansion(workspaceId, result.rootNodeIds, true);
    const openDepth = Math.max(0, Math.floor(args.openDepth ?? 1));
    if (openDepth > 1 && result.rootNodeIds.length > 0) {
      onExpansionMapChange(openDescendants(
        withWorkspaceOpen,
        result.rootNodeIds,
        openDepth,
        store.getTreeItems(workspaceId),
      ));
    }

    return { ...result, paperCount: papers.length, injectMs, projectMs };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runProjectWorkspacePapers, setWorkspacePapers, updateWorkspaceExpansion, onExpansionMapChange]);

  const rebuildWorkspacePaper = useCallback((workspaceId: string) => {
    if (store.isFullyLoaded(workspaceId)) {
      setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId));
    } else {
      setWorkspacePapers(workspaceId, [buildWsPaper(workspaceId, [])]);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildWsPaper, runProjectWorkspacePapers, setWorkspacePapers]);

  // reprojectWorkspace re-runs the full projection the way every subtree
  // expansion and every treeChanged refresh does, and reports how long it took.
  // projectWorkspacePapers rebuilds *all* papers for the workspace, so this is
  // the per-interaction cost that grows with item count — the load test measures
  // it directly rather than inferring it from frame timings.
  const reprojectWorkspace = useCallback((workspaceId: string, iterations = 1) => {
    const samples: number[] = [];
    let papers: Paper[] = [];
    for (let i = 0; i < Math.max(1, iterations); i++) {
      const startedAt = performance.now();
      papers = runProjectWorkspacePapers(workspaceId);
      samples.push(performance.now() - startedAt);
    }
    setWorkspacePapers(workspaceId, papers);
    const sorted = [...samples].sort((a, b) => a - b);
    return {
      paperCount: papers.length,
      iterations: samples.length,
      medianMs: sorted[Math.floor(sorted.length / 2)],
      minMs: sorted[0],
      maxMs: sorted[sorted.length - 1],
      samples,
    };
  }, [runProjectWorkspacePapers, setWorkspacePapers]);

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
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canReadWorkspaceTree, runProjectWorkspacePapers, setWorkspacePapers]);

  useEffect(() => {
    loadSubtreeAndProjectRef.current = loadSubtreeAndProject;
  }, [loadSubtreeAndProject]);

  const handleOpenWorkspace = useCallback(async (workspaceId: string, overrideWorkspace?: Workspace) => {
    if (overrideWorkspace) {
      store.rememberNewlyCreated(overrideWorkspace);
    }
    // Already fetched: re-project from cache and reveal, no network.
    if (store.isFullyLoaded(workspaceId)) {
      setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId));
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
  }, [buildWsPaper, canReadWorkspaceTree, onFocusedNodeIdChange, refreshWorkspaceTree, runProjectWorkspacePapers, setWorkspacePapers, updateWorkspaceExpansion]);

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
    injectMockNode,
    injectMockWorkspaceTree,
    reprojectWorkspace,
    getDebugSnapshot: store.debugSnapshot,
    resetTree,
    buildWsPaper,
    rebuildWorkspacePaper,
    uploadWorkspaceFile: handleUploadWorkspaceFile,
  };
}
