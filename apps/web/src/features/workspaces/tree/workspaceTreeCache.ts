import { create } from '@bufbuild/protobuf';
import { SubtreeItemSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import { findDocumentRootItemIds, findRootItemId } from '@/features/tree/buildTree';
import type { ApiItem, SubtreeItem } from '@/features/tree/api';
import type { Workspace } from '@/features/workspaces/api';
import type { MergeDocumentRootResult, RefreshResult, TreeStoreDebugSnapshot, WorkspaceTreeCache } from './workspaceTreeTypes';

export function createWorkspaceTreeCache(): WorkspaceTreeCache {
  const itemWorkspace = new Map<string, string>();
  const itemHasChildren = new Map<string, boolean>();
  const workspaceRootItems = new Map<string, string>();
  const workspaceDocumentRootIds = new Map<string, string[]>();
  const workspaceTreeItems = new Map<string, Map<string, SubtreeItem>>();
  const loadedSubtreeItems = new Set<string>();
  const loadingSubtreeItems = new Set<string>();
  const fullyLoadedWorkspaces = new Set<string>();
  const initializedWorkspaces = new Set<string>();
  const newlyCreatedWorkspaces = new Map<string, Workspace>();

  const mergeItemsIntoCache = (workspaceId: string, items: SubtreeItem[]) => {
    const workspaceItems = workspaceTreeItems.get(workspaceId) ?? new Map<string, SubtreeItem>();
    workspaceTreeItems.set(workspaceId, workspaceItems);
    for (const item of items) {
      const id = item.item!.id;
      workspaceItems.set(id, item);
      itemWorkspace.set(id, workspaceId);
      itemHasChildren.set(id, item.hasChildren);
    }
  };

  return {
    getRootItemId: (workspaceId) => workspaceRootItems.get(workspaceId),
    getDocumentRootIds: (workspaceId) => workspaceDocumentRootIds.get(workspaceId) ?? [],
    getTreeItems: (workspaceId) => workspaceTreeItems.get(workspaceId) ?? new Map(),
    getItemWorkspaceId: (itemId) => itemWorkspace.get(itemId),
    hasInitialized: (workspaceId) => initializedWorkspaces.has(workspaceId),
    isLoaded: (itemId) => loadedSubtreeItems.has(itemId),
    isLoading: (itemId) => loadingSubtreeItems.has(itemId),
    hasChildren: (itemId) => itemHasChildren.get(itemId) === true,
    isFullyLoaded: (workspaceId) => fullyLoadedWorkspaces.has(workspaceId),
    getNewlyCreated: (workspaceId) => newlyCreatedWorkspaces.get(workspaceId),
    listNewlyCreatedIds: () => Array.from(newlyCreatedWorkspaces.keys()),
    markInitialized: (workspaceId) => initializedWorkspaces.add(workspaceId),
    rememberNewlyCreated: (workspace) => {
      newlyCreatedWorkspaces.set(workspace.workspaceId, workspace);
    },
    pruneNewlyCreated: (workspaces) => {
      for (const ws of workspaces) {
        newlyCreatedWorkspaces.delete(ws.workspaceId);
      }
    },
    replaceWorkspaceTree: (workspaceId: string, items: ApiItem[]): RefreshResult => {
      const rootItemId = findRootItemId(items);
      if (!rootItemId) {
        throw new Error(`workspace ${workspaceId} has no workspace_root item`);
      }

      const previousDocumentRootIds = workspaceDocumentRootIds.get(workspaceId) ?? [];
      const documentRootIds = findDocumentRootItemIds(items);
      const newDocumentRootIds = documentRootIds.filter((id) => !previousDocumentRootIds.includes(id));

      workspaceRootItems.set(workspaceId, rootItemId);
      workspaceDocumentRootIds.set(workspaceId, documentRootIds);

      const treeItems = new Map<string, SubtreeItem>();
      workspaceTreeItems.set(workspaceId, treeItems);
      for (const item of items) {
        const hasChildren = item.childIds.length > 0;
        itemWorkspace.set(item.id, workspaceId);
        itemHasChildren.set(item.id, hasChildren);
        treeItems.set(item.id, create(SubtreeItemSchema, { item, hasChildren }));
      }

      fullyLoadedWorkspaces.add(workspaceId);
      for (const item of items) {
        loadedSubtreeItems.add(item.id);
      }

      return { rootItemId, documentRootIds, newDocumentRootIds };
    },
    shouldSkipSubtreeLoad: (workspaceId, itemId) =>
      fullyLoadedWorkspaces.has(workspaceId) || loadingSubtreeItems.has(itemId) || loadedSubtreeItems.has(itemId),
    markSubtreeLoading: (itemId) => loadingSubtreeItems.add(itemId),
    markSubtreeLoaded: (itemId) => loadedSubtreeItems.add(itemId),
    markSubtreeLoadFinished: (itemId) => loadingSubtreeItems.delete(itemId),
    mergeSubtreeItems: mergeItemsIntoCache,
    mergeDocumentRootItems: (
      workspaceId: string,
      createdDocumentRootItemId: string,
      items: SubtreeItem[],
    ): MergeDocumentRootResult | null => {
      const workspaceRootItemId = workspaceRootItems.get(workspaceId);
      if (!workspaceRootItemId || items.length === 0) return null;

      const previousDocumentRootIds = workspaceDocumentRootIds.get(workspaceId) ?? [];
      if (!previousDocumentRootIds.includes(createdDocumentRootItemId)) {
        workspaceDocumentRootIds.set(workspaceId, [
          ...previousDocumentRootIds,
          createdDocumentRootItemId,
        ]);
      }
      for (const item of items) {
        loadedSubtreeItems.add(item.item!.id);
      }
      mergeItemsIntoCache(workspaceId, items);
      return { workspaceRootItemId, items };
    },
    debugSnapshot: (workspaceId: string): TreeStoreDebugSnapshot => {
      const treeItems = workspaceTreeItems.get(workspaceId) ?? new Map<string, SubtreeItem>();
      const workspaceItemIds = new Set(treeItems.keys());
      return {
        workspaceId,
        workspaceRootItemId: workspaceRootItems.get(workspaceId),
        documentRootIds: workspaceDocumentRootIds.get(workspaceId) ?? [],
        initialized: initializedWorkspaces.has(workspaceId),
        fullyLoaded: fullyLoadedWorkspaces.has(workspaceId),
        itemCount: treeItems.size,
        treeItems: Array.from(treeItems.values()),
        loadedItemIds: Array.from(loadedSubtreeItems).filter((id) => workspaceItemIds.has(id)),
        loadingItemIds: Array.from(loadingSubtreeItems).filter((id) => workspaceItemIds.has(id)),
      };
    },
    reset: () => {
      itemWorkspace.clear();
      itemHasChildren.clear();
      workspaceRootItems.clear();
      workspaceDocumentRootIds.clear();
      workspaceTreeItems.clear();
      loadedSubtreeItems.clear();
      loadingSubtreeItems.clear();
      fullyLoadedWorkspaces.clear();
      initializedWorkspaces.clear();
      newlyCreatedWorkspaces.clear();
    },
  };
}
