'use client';

import { useCallback, useEffect, useRef } from 'react';
import { create } from '@bufbuild/protobuf';
import { SubtreeItemSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import { findDocumentRootItemIds, findRootItemId } from '@/features/tree/buildTree';
import { getSubtree, getTree, type SubtreeItem } from '@/features/tree/api';
import type { Workspace } from '@/features/workspaces/api';

export interface RefreshResult {
  rootItemId: string;
  documentRootIds: string[];
  newDocumentRootIds: string[];
}

export interface MergeDocumentRootResult {
  workspaceRootItemId: string;
  items: SubtreeItem[];
}

export interface TreeStore {
  // getters
  getRootItemId: (workspaceId: string) => string | undefined;
  getDocumentRootIds: (workspaceId: string) => string[];
  getTreeItems: (workspaceId: string) => Map<string, SubtreeItem>;
  getItemWorkspaceId: (itemId: string) => string | undefined;
  hasInitialized: (workspaceId: string) => boolean;
  isLoaded: (itemId: string) => boolean;
  isLoading: (itemId: string) => boolean;
  hasChildren: (itemId: string) => boolean;
  isFullyLoaded: (workspaceId: string) => boolean;
  getNewlyCreated: (workspaceId: string) => Workspace | undefined;
  listNewlyCreatedIds: () => string[];
  // mutators
  markInitialized: (workspaceId: string) => void;
  rememberNewlyCreated: (workspace: Workspace) => void;
  // tree ops
  refreshWorkspaceTree: (workspaceId: string) => Promise<RefreshResult>;
  loadSubtree: (workspaceId: string, itemId: string, maxDepth?: number) => Promise<SubtreeItem[]>;
  mergeDocumentRoot: (
    workspaceId: string,
    createdDocumentRootItemId: string,
  ) => Promise<MergeDocumentRootResult | null>;
  reset: () => void;
}

// useTreeStore is the data layer for workspace trees. It owns the Map/Set
// refs that cache items, document_root ids, and load state. Methods return
// pure data; the orchestrator is responsible for projecting it into Papers
// and for updating the expansion map.
export function useTreeStore(workspaces: Workspace[]): TreeStore {
  const itemWorkspaceRef = useRef<Map<string, string>>(new Map());
  const itemHasChildrenRef = useRef<Map<string, boolean>>(new Map());
  const workspaceRootItemRef = useRef<Map<string, string>>(new Map());
  const workspaceDocumentRootIdsRef = useRef<Map<string, string[]>>(new Map());
  const workspaceTreeItemsRef = useRef<Map<string, Map<string, SubtreeItem>>>(new Map());
  const loadedSubtreeItemsRef = useRef<Set<string>>(new Set());
  const loadingSubtreeItemsRef = useRef<Set<string>>(new Set());
  const fullyLoadedWorkspacesRef = useRef<Set<string>>(new Set());
  const initializedWorkspacesRef = useRef<Set<string>>(new Set());
  const newlyCreatedWorkspacesRef = useRef<Map<string, Workspace>>(new Map());

  // Once a newly created workspace appears in the official list, drop the
  // optimistic copy. Holding it around indefinitely would mask deletions.
  useEffect(() => {
    if (workspaces.length === 0) return;
    for (const ws of workspaces) {
      newlyCreatedWorkspacesRef.current.delete(ws.workspaceId);
    }
  }, [workspaces]);

  const mergeItemsIntoCache = (workspaceId: string, items: SubtreeItem[]) => {
    const workspaceItems = workspaceTreeItemsRef.current.get(workspaceId) ?? new Map<string, SubtreeItem>();
    workspaceTreeItemsRef.current.set(workspaceId, workspaceItems);
    for (const item of items) {
      const id = item.item!.id;
      workspaceItems.set(id, item);
      itemWorkspaceRef.current.set(id, workspaceId);
      itemHasChildrenRef.current.set(id, item.hasChildren);
    }
  };

  const refreshWorkspaceTree = useCallback(async (workspaceId: string): Promise<RefreshResult> => {
    const tree = await getTree(workspaceId);
    const items = tree.items;
    // workspace_root is created atomically with the workspace, so a missing
    // root or an empty items array means an upstream invariant is broken.
    const rootItemId = findRootItemId(items);
    if (!rootItemId) {
      throw new Error(`workspace ${workspaceId} has no workspace_root item`);
    }

    const previousDocumentRootIds = workspaceDocumentRootIdsRef.current.get(workspaceId) ?? [];
    const documentRootIds = findDocumentRootItemIds(items);
    const newDocumentRootIds = documentRootIds.filter((id) => !previousDocumentRootIds.includes(id));

    workspaceRootItemRef.current.set(workspaceId, rootItemId);
    workspaceDocumentRootIdsRef.current.set(workspaceId, documentRootIds);

    const treeItems = new Map<string, SubtreeItem>();
    workspaceTreeItemsRef.current.set(workspaceId, treeItems);
    for (const item of items) {
      const hasChildren = item.childIds.length > 0;
      itemWorkspaceRef.current.set(item.id, workspaceId);
      itemHasChildrenRef.current.set(item.id, hasChildren);
      treeItems.set(item.id, create(SubtreeItemSchema, { item, hasChildren }));
    }

    fullyLoadedWorkspacesRef.current.add(workspaceId);
    for (const item of items) {
      loadedSubtreeItemsRef.current.add(item.id);
    }

    return { rootItemId, documentRootIds, newDocumentRootIds };
  }, []);

  // loadSubtree fetches a subtree under itemId and merges its items into the
  // cache. The orchestrator decides whether to recursively load children
  // that are already expanded — the store does not know about openness.
  const loadSubtree = useCallback(async (
    workspaceId: string,
    itemId: string,
    maxDepth = 1,
  ): Promise<SubtreeItem[]> => {
    if (fullyLoadedWorkspacesRef.current.has(workspaceId)) return [];
    if (loadingSubtreeItemsRef.current.has(itemId) || loadedSubtreeItemsRef.current.has(itemId)) return [];
    loadingSubtreeItemsRef.current.add(itemId);
    try {
      const items = await getSubtree(workspaceId, itemId, maxDepth);
      mergeItemsIntoCache(workspaceId, items);
      loadedSubtreeItemsRef.current.add(itemId);
      return items;
    } catch (err) {
      console.error('Failed to load subtree:', err);
      return [];
    } finally {
      loadingSubtreeItemsRef.current.delete(itemId);
    }
  }, []);

  const mergeDocumentRoot = useCallback(async (
    workspaceId: string,
    createdDocumentRootItemId: string,
  ): Promise<MergeDocumentRootResult | null> => {
    const workspaceRootItemId = workspaceRootItemRef.current.get(workspaceId);
    if (!workspaceRootItemId) return null;
    const items = await getSubtree(workspaceId, createdDocumentRootItemId, 5);
    if (items.length === 0) return null;

    const previousDocumentRootIds = workspaceDocumentRootIdsRef.current.get(workspaceId) ?? [];
    if (!previousDocumentRootIds.includes(createdDocumentRootItemId)) {
      workspaceDocumentRootIdsRef.current.set(workspaceId, [
        ...previousDocumentRootIds,
        createdDocumentRootItemId,
      ]);
    }
    for (const item of items) {
      loadedSubtreeItemsRef.current.add(item.item!.id);
    }
    mergeItemsIntoCache(workspaceId, items);
    return { workspaceRootItemId, items };
  }, []);

  const reset = useCallback(() => {
    itemWorkspaceRef.current.clear();
    itemHasChildrenRef.current.clear();
    workspaceRootItemRef.current.clear();
    workspaceDocumentRootIdsRef.current.clear();
    workspaceTreeItemsRef.current.clear();
    loadedSubtreeItemsRef.current.clear();
    loadingSubtreeItemsRef.current.clear();
    fullyLoadedWorkspacesRef.current.clear();
    initializedWorkspacesRef.current.clear();
  }, []);

  return {
    getRootItemId: (workspaceId) => workspaceRootItemRef.current.get(workspaceId),
    getDocumentRootIds: (workspaceId) => workspaceDocumentRootIdsRef.current.get(workspaceId) ?? [],
    getTreeItems: (workspaceId) => workspaceTreeItemsRef.current.get(workspaceId) ?? new Map(),
    getItemWorkspaceId: (itemId) => itemWorkspaceRef.current.get(itemId),
    hasInitialized: (workspaceId) => initializedWorkspacesRef.current.has(workspaceId),
    isLoaded: (itemId) => loadedSubtreeItemsRef.current.has(itemId),
    isLoading: (itemId) => loadingSubtreeItemsRef.current.has(itemId),
    hasChildren: (itemId) => itemHasChildrenRef.current.get(itemId) === true,
    isFullyLoaded: (workspaceId) => fullyLoadedWorkspacesRef.current.has(workspaceId),
    getNewlyCreated: (workspaceId) => newlyCreatedWorkspacesRef.current.get(workspaceId),
    listNewlyCreatedIds: () => Array.from(newlyCreatedWorkspacesRef.current.keys()),
    markInitialized: (workspaceId) => initializedWorkspacesRef.current.add(workspaceId),
    rememberNewlyCreated: (workspace) => {
      newlyCreatedWorkspacesRef.current.set(workspace.workspaceId, workspace);
    },
    refreshWorkspaceTree,
    loadSubtree,
    mergeDocumentRoot,
    reset,
  };
}
