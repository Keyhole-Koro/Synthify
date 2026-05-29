'use client';

import { useCallback, useEffect, useMemo, useRef } from 'react';
import type { ExpansionMap, Paper } from '@keyhole-koro/paper-in-paper';
import { projectWorkspacePapers } from '@/features/workspaces/useWorkspaceProjection';
import { ROOT_ID, WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import { type Workspace } from '@/features/workspaces/api';
import type { WorkspacePaperRuntimeState } from '@/features/workspaces/WorkspacePaper';
import { useTreeStore } from './tree/useTreeStore';
import { useWorkspaceUpload } from './tree/useWorkspaceUpload';
import { useExpansionWatcher } from './tree/useExpansionWatcher';
import { createWorkspacePaperFactory } from './tree/workspacePaperFactory';
export type { WorkspacePaperRuntimeState } from '@/features/workspaces/WorkspacePaper';

// useWorkspaceTree is the orchestrator. It owns no data of its own — the
// tree cache lives in useTreeStore, the paper factory in
// workspacePaperFactory, and the expansion side effects in
// useExpansionWatcher. Its job is to wire those together: pass refresh
// results through projection to setWorkspacePapers, kick recursive subtree
// loads for items the user already had open, and keep the expansion map in
// sync with the cache.
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
) {
  const expansionMapRef = useRef<ExpansionMap>(expansionMap);
  const workspacesRef = useRef<Workspace[]>(workspaces);
  useEffect(() => {
    expansionMapRef.current = expansionMap;
  }, [expansionMap]);
  useEffect(() => {
    workspacesRef.current = workspaces;
  }, [workspaces]);

  const store = useTreeStore(workspaces);
  const handleUploadWorkspaceFile = useWorkspaceUpload();

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

  // getWorkspace / onProcessingComplete are stable callbacks that read live
  // values (workspacesRef and mergeDocumentRootIntoTreeRef) at call time, so
  // the factory itself stays a pure function over stable inputs.
  const getWorkspace = useCallback(
    (id: string) =>
      workspacesRef.current.find((w) => w.workspaceId === id) ?? store.getNewlyCreated(id),
  // store getter is stable.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  [],
  );

  const onProcessingComplete = useCallback(async (workspaceId: string, createdDocumentRootItemId: string) => {
    onWorkspaceRuntimeStateComplete(workspaceId);
    await mergeDocumentRootIntoTreeRef.current(workspaceId, createdDocumentRootItemId);
  }, [onWorkspaceRuntimeStateComplete]);

  // The factory reads getWorkspace / onProcessingComplete at render time
  // only as closures — the actual ref dereference happens later, inside the
  // returned buildWsPaper invocation. react-hooks/refs cannot tell the
  // difference, so suppress it locally.
  const buildWsPaper = useMemo(
    // eslint-disable-next-line react-hooks/refs
    () => createWorkspacePaperFactory({
      getWorkspace,
      getWorkspaceName,
      getRuntimeState: getWorkspacePaperRuntimeState,
      onUploadFile: handleUploadWorkspaceFile,
      onRenameWorkspace,
      onSuggestedWorkspaceName,
      onProcessingComplete,
    }),
    [getWorkspace, getWorkspaceName, getWorkspacePaperRuntimeState, handleUploadWorkspaceFile,
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

  // mergeDocumentRootIntoTree handles a PROCESS_DOCUMENT completion: store
  // merges the new subtree, orchestrator re-projects and reveals the new
  // document_root in the expansion map.
  const mergeDocumentRootIntoTree = useCallback(async (
    workspaceId: string,
    createdDocumentRootItemId: string,
  ) => {
    const merged = await store.mergeDocumentRoot(workspaceId, createdDocumentRootItemId);
    if (!merged) return;
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, merged.workspaceRootItemId));
    updateWorkspaceExpansion(workspaceId, [createdDocumentRootItemId], true);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runProjectWorkspacePapers, setWorkspacePapers, updateWorkspaceExpansion]);

  useEffect(() => {
    mergeDocumentRootIntoTreeRef.current = mergeDocumentRootIntoTree;
  }, [mergeDocumentRootIntoTree]);

  const rebuildWorkspacePaper = useCallback((workspaceId: string) => {
    const rootItemId = store.getRootItemId(workspaceId);
    const childPapers = rootItemId
      ? store.getDocumentRootIds(workspaceId).map((id) => ({ id }))
      : [];
    setWorkspacePapers(workspaceId, [buildWsPaper(workspaceId, childPapers)]);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildWsPaper, setWorkspacePapers]);

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
  }, [runProjectWorkspacePapers, setWorkspacePapers]);

  useEffect(() => {
    loadSubtreeAndProjectRef.current = loadSubtreeAndProject;
  }, [loadSubtreeAndProject]);

  const refreshWorkspaceTree = useCallback(async (
    workspaceId: string,
    opts: { revealNewDocumentRoots?: boolean } = {},
  ) => {
    const { rootItemId, newDocumentRootIds } = await store.refreshWorkspaceTree(workspaceId);
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, rootItemId));
    updateWorkspaceExpansion(workspaceId, newDocumentRootIds, opts.revealNewDocumentRoots === true);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runProjectWorkspacePapers, setWorkspacePapers, updateWorkspaceExpansion]);

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
    await refreshWorkspaceTree(workspaceId);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildWsPaper, loadSubtreeAndProject, onFocusedNodeIdChange, refreshWorkspaceTree, setWorkspacePapers, updateWorkspaceExpansion]);

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
    resetTree,
    buildWsPaper,
    rebuildWorkspacePaper,
    uploadWorkspaceFile: handleUploadWorkspaceFile,
  };
}
