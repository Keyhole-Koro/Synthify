'use client';

import { useRef, useCallback, useEffect } from 'react';
import type { ExpansionMap, Paper } from '@keyhole-koro/paper-in-paper';
import { WorkspacePaper } from '@/features/workspaces/WorkspacePaper';
import type { WorkspacePaperRuntimeState } from '@/features/workspaces/WorkspacePaper';
import { findRootItemId, findDocumentRootItemIds } from '@/features/tree/buildTree';
import { projectWorkspacePapers } from '@/features/workspaces/useWorkspaceProjection';
import { getTree, getSubtree, type SubtreeItem } from '@/features/tree/api';
import { create } from '@bufbuild/protobuf';
import { SubtreeItemSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import { createDocument, startProcessing, uploadFile } from '@/features/documents/api';
import { ROOT_ID, WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import { type Workspace } from '@/features/workspaces/api';

export type { WorkspacePaperRuntimeState } from '@/features/workspaces/WorkspacePaper';

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

  // keep refs in sync so async callbacks always see latest value
  useEffect(() => {
    expansionMapRef.current = expansionMap;
  }, [expansionMap]);
  useEffect(() => {
    workspacesRef.current = workspaces;
  }, [workspaces]);

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
  const newlyCreatedWorkspacesRef = useRef<Map<string, Workspace>>(new Map());

  // Clean up newlyCreatedWorkspacesRef when they appear in the official workspaces list
  useEffect(() => {
    if (workspaces.length === 0) return;
    for (const ws of workspaces) {
      newlyCreatedWorkspacesRef.current.delete(ws.workspaceId);
    }
  }, [workspaces]);

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
    map = openChild(ROOT_ID, WORKSPACES_ID, map);
    map = openChild(WORKSPACES_ID, workspaceId, map);

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
    await uploadFile(created.uploadUrl, file, created.uploadMethod, created.uploadContentType);
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
    const workspace = workspacesRef.current.find((candidate) => candidate.workspaceId === workspaceId) ||
                      newlyCreatedWorkspacesRef.current.get(workspaceId);
    if (!workspace) {
      return {
        id: workspaceId,
        title: '不明なワークスペース',
        description: 'ワークスペースが見つかりません',
        hue: 0,
        parentId: WORKSPACES_ID,
        childIds: [],
        content: (
          <div className="flex flex-col gap-2 p-4 text-center">
            <p className="text-sm font-medium text-slate-500">ワークスペースが見つかりません。</p>
            <p className="text-xs text-slate-400">削除されたか、アクセス権限がない可能性があります。</p>
          </div>
        ),
      };
    }
    const workspaceName = workspace.name || getWorkspaceName(workspaceId);
    const runtimeState = getWorkspacePaperRuntimeState(workspaceId);
    const runtimeKey = [
      runtimeState.initialActiveJobId ?? '',
      runtimeState.initialDocumentId ?? '',
      runtimeState.initialUploading === true ? 'uploading' : '',
      runtimeState.initialUploadMessage ?? '',
    ].join(':');
    return {
      id: workspaceId,
      title: workspaceName,
      description: 'ドキュメントと知識構造',
      hue: 200,
      parentId: WORKSPACES_ID,
      childIds: childPapers.map((p) => p.id),
      content: (
        <WorkspacePaper
          key={`${workspaceId}:${runtimeKey}`}
          workspace={workspace}
          workspaceId={workspaceId}
          workspaceName={workspaceName}
          hasTree={childPapers.length > 0}
          childItems={childPapers}
          initialActiveJobId={runtimeState.initialActiveJobId ?? null}
          initialDocumentId={runtimeState.initialDocumentId ?? null}
          initialUploading={runtimeState.initialUploading}
          initialUploadMessage={runtimeState.initialUploadMessage}
          onUploadFile={(file) => handleUploadWorkspaceFile(workspaceId, file)}
          onRenameWorkspace={(name) => onRenameWorkspace(workspaceId, name)}
          onSuggestedWorkspaceName={(name) => onSuggestedWorkspaceName(workspaceId, name)}
          onProcessingComplete={async (_jobId, createdDocumentRootItemId) => {
            onWorkspaceRuntimeStateComplete(workspaceId);
            // The worker always reports the new document_root id alongside
            // job completion, so the merge path is the only one we need.
            await mergeDocumentRootIntoTree(workspaceId, createdDocumentRootItemId);
          }}
        />
      ),
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    getWorkspaceName,
    getWorkspacePaperRuntimeState,
    handleUploadWorkspaceFile,
    onRenameWorkspace,
    onSuggestedWorkspaceName,
    onWorkspaceRuntimeStateComplete,
  ]);

  function runProjectWorkspacePapers(workspaceId: string, workspaceRootItemId: string): Paper[] {
    const treeItems = workspaceTreeItemsRef.current.get(workspaceId) ?? new Map();
    const documentRootIds = workspaceDocumentRootIdsRef.current.get(workspaceId) ?? [];

    initializedWorkspacesRef.current.add(workspaceId);
    return projectWorkspacePapers(workspaceId, workspaceRootItemId, treeItems, documentRootIds, buildWsPaper);
  }

  const rebuildWorkspacePaper = useCallback((workspaceId: string) => {
    const rootItemId = workspaceRootItemRef.current.get(workspaceId);
    const childPapers = rootItemId
      ? (workspaceDocumentRootIdsRef.current.get(workspaceId) ?? []).map((id) => ({ id }))
      : [];
    setWorkspacePapers(workspaceId, [buildWsPaper(workspaceId, childPapers)]);
  }, [buildWsPaper, setWorkspacePapers]);

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

  // mergeDocumentRootIntoTree is the incremental refresh path used after a
  // PROCESS_DOCUMENT job completes: it pulls just the freshly created
  // document_root's subtree and grafts it under the workspace root rather
  // than refetching the whole workspace tree.
  async function mergeDocumentRootIntoTree(
    workspaceId: string,
    createdDocumentRootItemId: string,
  ): Promise<void> {
    const workspaceRootItemId = workspaceRootItemRef.current.get(workspaceId);
    if (!workspaceRootItemId) return;
    const items = await getSubtree(workspaceId, createdDocumentRootItemId, 5);
    if (items.length === 0) return;

    const previousDocumentRootIds = workspaceDocumentRootIdsRef.current.get(workspaceId) ?? [];
    if (!previousDocumentRootIds.includes(createdDocumentRootItemId)) {
      workspaceDocumentRootIdsRef.current.set(workspaceId, [
        ...previousDocumentRootIds,
        createdDocumentRootItemId,
      ]);
    }
    for (const item of items) {
      const id = item.item!.id;
      itemWorkspaceRef.current.set(id, workspaceId);
      itemHasChildrenRef.current.set(id, item.hasChildren);
      loadedSubtreeItemsRef.current.add(id);
    }
    void mergeTreeIntoWorkspace(workspaceId, workspaceRootItemId, items);
    setWorkspacePapers(workspaceId, runProjectWorkspacePapers(workspaceId, workspaceRootItemId));
    updateWorkspaceExpansion(workspaceId, [createdDocumentRootItemId], true);
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
    const rootItemId = findRootItemId(items);
    if (!rootItemId) return;

    const previousDocumentRootIds = workspaceDocumentRootIdsRef.current.get(workspaceId) ?? [];
    const documentRootIds = findDocumentRootItemIds(items);
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
        rebuildWorkspacePaper(workspaceId);
      }
    }

    const rootOpenIds = expansionMap.get(ROOT_ID)?.openChildIds ?? [];
    if (!rootOpenIds.includes(WORKSPACES_ID)) return;

    const validWorkspaceIds = new Set([
      ...workspaces.map((workspace) => workspace.workspaceId),
      ...newlyCreatedWorkspacesRef.current.keys(),
    ]);
    const workspacesOpenIds = expansionMap.get(WORKSPACES_ID)?.openChildIds ?? [];
    const validOpenWorkspaceIds = workspacesOpenIds.filter((workspaceId) => validWorkspaceIds.has(workspaceId));
    if (validOpenWorkspaceIds.length !== workspacesOpenIds.length) {
      onExpansionMapChange(setOpenChildren(WORKSPACES_ID, validOpenWorkspaceIds, expansionMap));
    }

    for (const workspaceId of validOpenWorkspaceIds) {
      if (!initializedWorkspacesRef.current.has(workspaceId)) {
        void handleOpenWorkspace(workspaceId);
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaces]);

  const handleOpenWorkspace = useCallback(async (workspaceId: string, overrideWorkspace?: Workspace) => {
    if (overrideWorkspace) {
      newlyCreatedWorkspacesRef.current.set(workspaceId, overrideWorkspace);
    }
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
  }, [buildWsPaper, onFocusedNodeIdChange, refreshWorkspaceTree]);

  const resetTree = useCallback(() => {
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
