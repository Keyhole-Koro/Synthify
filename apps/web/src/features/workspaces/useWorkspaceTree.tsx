'use client';

import { useRef, useCallback, useEffect } from 'react';
import type { ExpansionMap, Paper } from '@keyhole-koro/paper-in-paper';
import { WorkspacePaper } from '@/features/workspaces/WorkspacePaper';
import { findRootItemId } from '@/features/tree/buildTree';
import { projectWorkspacePapers } from '@/features/workspaces/useWorkspaceProjection';
import { getTree, getSubtree, type ApiItem, type SubtreeItem } from '@/features/tree/api';
import { create } from '@bufbuild/protobuf';
import { SubtreeItemSchema } from '@synthify/proto-ts/gen/synthify/tree/v1/tree_types_pb';
import { createDocument, startProcessing, uploadFile } from '@/features/documents/api';
import { ROOT_ID } from '@/features/paperMap/staticPapers';
import { type Workspace } from '@/features/workspaces/api';

export function useWorkspaceTree(
  getWorkspaceName: (id: string) => string,
  expansionMap: ExpansionMap,
  onExpansionMapChange: (expansionMap: ExpansionMap) => void,
  onFocusedNodeIdChange: (nodeId: string | null) => void,
  setWorkspacePapers: (workspaceId: string, papers: Paper[]) => void,
  clearWorkspacePapers: () => void,
  workspaces: Workspace[],
) {
  const expansionMapRef = useRef<ExpansionMap>(expansionMap);

  // keep ref in sync so async callbacks always see latest value
  useEffect(() => {
    expansionMapRef.current = expansionMap;
  }, [expansionMap]);

  const itemWorkspaceRef = useRef<Map<string, string>>(new Map());
  const itemHasChildrenRef = useRef<Map<string, boolean>>(new Map());
  const workspaceRootItemRef = useRef<Map<string, string>>(new Map());
  const workspaceDocumentRootIdsRef = useRef<Map<string, string[]>>(new Map());
  const workspaceTreeItemsRef = useRef<Map<string, Map<string, SubtreeItem>>>(new Map());
  const loadedSubtreeItemsRef = useRef<Set<string>>(new Set());
  const loadingSubtreeItemsRef = useRef<Set<string>>(new Set());
  const fullyLoadedWorkspacesRef = useRef<Set<string>>(new Set());
  const prevExpansionRef = useRef<ExpansionMap>(new Map());
  const initializedWorkspacesRef = useRef<Set<string>>(new Set());

  function setOpenChildren(parentId: string, childIds: string[], base: ExpansionMap): ExpansionMap {
    const next = new Map(base);
    next.set(parentId, { openChildIds: childIds });
    return next;
  }

  function openChild(parentId: string, childId: string, base: ExpansionMap): ExpansionMap {
    const current = base.get(parentId)?.openChildIds ?? [];
    if (current.includes(childId)) return base;
    return setOpenChildren(parentId, [...current, childId], base);
  }

  function updateWorkspaceExpansion(
    workspaceId: string,
    newDocumentRootIds: string[] = [],
    revealNewDocumentRoots = false,
  ) {
    let map = expansionMapRef.current;
    map = openChild(ROOT_ID, 'workspaces', map);
    map = openChild('workspaces', workspaceId, map);

    const allTreeItemIds = new Set(workspaceTreeItemsRef.current.get(workspaceId)?.keys() ?? []);
    const currentWorkspaceOpenIds = (map.get(workspaceId)?.openChildIds ?? []).filter((id) => allTreeItemIds.has(id));
    const openChildIds = revealNewDocumentRoots
      ? Array.from(new Set([...currentWorkspaceOpenIds, ...newDocumentRootIds]))
      : currentWorkspaceOpenIds;

    map = setOpenChildren(workspaceId, openChildIds, map);
    onExpansionMapChange(map);
  }

  const handleUploadWorkspaceFile = useCallback(async (workspaceId: string, file: File) => {
    const created = await createDocument(
      workspaceId,
      file.name,
      file.type || 'application/octet-stream',
      file.size,
    );
    await uploadFile(created.uploadUrl, file, created.uploadMethod);
    const processing = await startProcessing(created.document.documentId);
    return {
      jobId: processing.job.jobId,
      documentId: created.document.documentId,
    };
  }, []);

  const buildWsPaper = useCallback((
    workspaceId: string,
    childPapers: { id: string }[],
  ): Paper => {
    const workspaceName = getWorkspaceName(workspaceId);
    const workspace = workspaces.find((candidate) => candidate.workspaceId === workspaceId);
    if (!workspace) {
      throw new Error(`workspace not found: ${workspaceId}`);
    }
    return {
      id: workspaceId,
      title: workspaceName,
      description: 'ドキュメントと知識構造',
      hue: 200,
      parentId: 'workspaces',
      childIds: childPapers.map((p) => p.id),
      content: (
        <WorkspacePaper
          workspace={workspace}
          workspaceId={workspaceId}
          workspaceName={workspaceName}
          hasTree={childPapers.length > 0}
          childItems={childPapers}
          onUploadFile={(file) => handleUploadWorkspaceFile(workspaceId, file)}
          onProcessingComplete={() => refreshWorkspaceTree(workspaceId, { revealNewDocumentRoots: true })}
        />
      ),
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [getWorkspaceName, handleUploadWorkspaceFile, workspaces]);

  function runProjectWorkspacePapers(workspaceId: string, workspaceRootItemId: string): Paper[] {
    const treeItems = workspaceTreeItemsRef.current.get(workspaceId) ?? new Map();
    const documentRootIds = workspaceDocumentRootIdsRef.current.get(workspaceId) ?? [];

    initializedWorkspacesRef.current.add(workspaceId);
    return projectWorkspacePapers(workspaceId, workspaceRootItemId, treeItems, documentRootIds, buildWsPaper);
  }

  async function mergeTreeIntoWorkspace(workspaceId: string, workspaceRootItemId: string, items: SubtreeItem[]) {
    const workspaceItems = workspaceTreeItemsRef.current.get(workspaceId) ?? new Map<string, SubtreeItem>();
    workspaceTreeItemsRef.current.set(workspaceId, workspaceItems);

    for (const item of items) {
      const id = item.item!.id;
      workspaceItems.set(id, item);

      if (item.hasChildren && (expansionMapRef.current.get(id)?.openChildIds.length ?? 0) > 0) {
        if (!loadedSubtreeItemsRef.current.has(id) && !loadingSubtreeItemsRef.current.has(id)) {
          void loadSubtreeForItem(workspaceId, workspaceRootItemId, id, 1);
        }
      }
    }

    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, workspaceRootItemId));
  }

  async function loadSubtreeForItem(workspaceId: string, workspaceRootItemId: string, itemId: string, maxDepth = 1) {
    if (fullyLoadedWorkspacesRef.current.has(workspaceId)) return;
    if (loadingSubtreeItemsRef.current.has(itemId) || loadedSubtreeItemsRef.current.has(itemId)) return;
    loadingSubtreeItemsRef.current.add(itemId);
    try {
      const items = await getSubtree(workspaceId, itemId, maxDepth);
      for (const item of items) {
        itemWorkspaceRef.current.set(item.item!.id, workspaceId);
        itemHasChildrenRef.current.set(item.item!.id, item.hasChildren);
      }
      mergeTreeIntoWorkspace(workspaceId, workspaceRootItemId, items);
      loadedSubtreeItemsRef.current.add(itemId);
    } catch (err) {
      console.error('Failed to load subtree:', err);
    } finally {
      loadingSubtreeItemsRef.current.delete(itemId);
    }
  }

  async function refreshWorkspaceTree(
    workspaceId: string,
    opts: { revealNewDocumentRoots?: boolean } = {},
  ) {
    const tree = await getTree(workspaceId);
    const items = tree?.items ?? [];
    if (items.length === 0) {
      workspaceRootItemRef.current.delete(workspaceId);
      workspaceDocumentRootIdsRef.current.set(workspaceId, []);
      workspaceTreeItemsRef.current.set(workspaceId, new Map());
      fullyLoadedWorkspacesRef.current.delete(workspaceId);
      setWorkspacePapers(workspaceId, [buildWsPaper(workspaceId, [])]);
      return;
    }
    const rootItemId = findRootItemId(items) ?? items[0]?.id;
    if (!rootItemId) return;

    const previousDocumentRootIds = workspaceDocumentRootIdsRef.current.get(workspaceId) ?? [];
    const rootItem = items.find((item: ApiItem) => item.id === rootItemId);
    const documentRootIds = rootItem?.childIds ?? [];
    const newDocumentRootIds = documentRootIds.filter((id: string) => !previousDocumentRootIds.includes(id));

    workspaceRootItemRef.current.set(workspaceId, rootItemId);
    workspaceDocumentRootIdsRef.current.set(workspaceId, documentRootIds);

    const treeItems = new Map<string, SubtreeItem>();
    workspaceTreeItemsRef.current.set(workspaceId, treeItems);
    for (const item of items) {
      const hasChildren = (item.childIds?.length ?? 0) > 0;
      itemWorkspaceRef.current.set(item.id, workspaceId);
      itemHasChildrenRef.current.set(item.id, hasChildren);
      treeItems.set(item.id, create(SubtreeItemSchema, { item, hasChildren }));
    }

    fullyLoadedWorkspacesRef.current.add(workspaceId);
    for (const item of items) {
      loadedSubtreeItemsRef.current.add(item.id);
    }
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, rootItemId));
    updateWorkspaceExpansion(workspaceId, newDocumentRootIds, opts.revealNewDocumentRoots === true);
  }

  // Watch expansionMap changes and load subtrees for newly opened items.
  useEffect(() => {
    const prev = prevExpansionRef.current;
    if (expansionMap === prev) return;

    const newlyOpened: string[] = [];
    for (const [parentId, entry] of expansionMap) {
      const currentIds = entry?.openChildIds ?? [];
      const prevIds = prev.get(parentId)?.openChildIds ?? [];
      const prevSet = new Set(prevIds);
      for (const childId of currentIds) {
        if (!prevSet.has(childId)) newlyOpened.push(childId);
      }
    }
    prevExpansionRef.current = expansionMap;

    for (const itemId of newlyOpened) {
      if (!itemHasChildrenRef.current.get(itemId)) continue;
      const workspaceId = itemWorkspaceRef.current.get(itemId);
      if (!workspaceId) continue;
      const workspaceRootItemId = workspaceRootItemRef.current.get(workspaceId);
      if (!workspaceRootItemId) continue;
      void loadSubtreeForItem(workspaceId, workspaceRootItemId, itemId, 1);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [expansionMap]);

  // Re-hydrate expanded workspaces on mount or when workspaces are loaded.
  useEffect(() => {
    if (workspaces.length === 0) return;

    for (const { workspaceId } of workspaces) {
      if (initializedWorkspacesRef.current.has(workspaceId)) {
        const rootItemId = workspaceRootItemRef.current.get(workspaceId);
        const childPapers = rootItemId
          ? (workspaceDocumentRootIdsRef.current.get(workspaceId) ?? []).map((id) => ({ id }))
          : [];
        setWorkspacePapers(workspaceId, [buildWsPaper(workspaceId, childPapers)]);
      }
    }

    const rootOpenIds = expansionMap.get(ROOT_ID)?.openChildIds ?? [];
    if (!rootOpenIds.includes('workspaces')) return;

    const workspacesOpenIds = expansionMap.get('workspaces')?.openChildIds ?? [];
    for (const workspaceId of workspacesOpenIds) {
      if (!initializedWorkspacesRef.current.has(workspaceId)) {
        void handleOpenWorkspace(workspaceId);
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaces]);

  const handleOpenWorkspace = useCallback(async (workspaceId: string) => {
    const knownRootId = workspaceRootItemRef.current.get(workspaceId);
    if (knownRootId) {
      if (!loadedSubtreeItemsRef.current.has(knownRootId)) {
        void loadSubtreeForItem(workspaceId, knownRootId, knownRootId, 1);
      }
      updateWorkspaceExpansion(workspaceId);
      onFocusedNodeIdChange(workspaceId);
      return;
    }
    initializedWorkspacesRef.current.add(workspaceId);
    setWorkspacePapers(workspaceId, [buildWsPaper(workspaceId, [])]);
    updateWorkspaceExpansion(workspaceId);
    onFocusedNodeIdChange(workspaceId);
    await refreshWorkspaceTree(workspaceId);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [getWorkspaceName, handleUploadWorkspaceFile]);

  function resetTree() {
    itemWorkspaceRef.current.clear();
    itemHasChildrenRef.current.clear();
    workspaceRootItemRef.current.clear();
    workspaceDocumentRootIdsRef.current.clear();
    workspaceTreeItemsRef.current.clear();
    loadedSubtreeItemsRef.current.clear();
    loadingSubtreeItemsRef.current.clear();
    fullyLoadedWorkspacesRef.current.clear();
    prevExpansionRef.current = new Map();
    initializedWorkspacesRef.current.clear();
    clearWorkspacePapers();
  }

  return {
    handleOpenWorkspace,
    resetTree,
    buildWsPaper,
  };
}
