import { create } from '@bufbuild/protobuf';
import { ItemSchema, SubtreeItemSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import { findRootNodeIds } from '@/features/tree/buildTree';
import type { ApiItem, SubtreeItem } from '@/features/tree/api';
import type { Workspace } from '@/features/workspaces/api';
import type { InjectMockNodeArgs, InjectMockWorkspaceTreeArgs, RefreshResult, TreeStoreDebugSnapshot, WorkspaceTreeCache } from './workspaceTreeTypes';

function makeDebugItemId() {
  return `debug_node_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

// buildMockWorkspaceTreeItems assembles a complete ApiItem[] (N root nodes,
// each with M child nodes) entirely on the frontend. There is no
// workspace_root item — root nodes sit directly under the workspace
// (parent_id = ''). The shape mirrors what the backend returns so it can flow
// through replaceWorkspaceTree unchanged.
function buildMockWorkspaceTreeItems(
  workspaceId: string,
  args: InjectMockWorkspaceTreeArgs,
): ApiItem[] {
  const documentCount = Math.max(1, args.documentCount ?? 2);
  const nodesPerDocument = Math.max(0, args.nodesPerDocument ?? 3);
  const titles = args.documentTitles ?? [];

  const items: ApiItem[] = [];

  for (let d = 0; d < documentCount; d++) {
    const rootNodeId = `debug_root_node_${workspaceId}_${d}`;
    const childIds: string[] = [];

    for (let n = 0; n < nodesPerDocument; n++) {
      const nodeId = `${rootNodeId}_node_${n}`;
      childIds.push(nodeId);
      items.push(create(ItemSchema, {
        id: nodeId,
        parentId: rootNodeId,
        title: `論点 ${d + 1}-${n + 1}`,
        description: `Mock node ${n + 1} of group ${d + 1}`,
        content: `<p>Mock node ${n + 1} の本文。__synthifyDebug が生成したフロントエンド専用のダミーです。</p>`,
        level: 2,
        childIds: [],
      }));
    }

    items.push(create(ItemSchema, {
      id: rootNodeId,
      parentId: '',
      title: titles[d]?.trim() || `Mock ノード ${d + 1}`,
      description: `Mock root node ${d + 1}`,
      content: `<p>Mock ルートノード ${d + 1} の概要。</p>`,
      level: 0,
      childIds,
    }));
  }

  return items;
}

export function createWorkspaceTreeCache(): WorkspaceTreeCache {
  const itemWorkspace = new Map<string, string>();
  const itemHasChildren = new Map<string, boolean>();
  const workspaceRootNodeIds = new Map<string, string[]>();
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

  const replaceWorkspaceTree = (workspaceId: string, items: ApiItem[]): RefreshResult => {
    const previousRootNodeIds = workspaceRootNodeIds.get(workspaceId) ?? [];
    const rootNodeIds = findRootNodeIds(items);
    const newRootNodeIds = rootNodeIds.filter((id) => !previousRootNodeIds.includes(id));

    workspaceRootNodeIds.set(workspaceId, rootNodeIds);

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

    return { rootNodeIds, newRootNodeIds };
  };

  return {
    getRootNodeIds: (workspaceId) => workspaceRootNodeIds.get(workspaceId) ?? [],
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
    replaceWorkspaceTree,
    shouldSkipSubtreeLoad: (workspaceId, itemId) =>
      fullyLoadedWorkspaces.has(workspaceId) || loadingSubtreeItems.has(itemId) || loadedSubtreeItems.has(itemId),
    markSubtreeLoading: (itemId) => loadingSubtreeItems.add(itemId),
    markSubtreeLoaded: (itemId) => loadedSubtreeItems.add(itemId),
    markSubtreeLoadFinished: (itemId) => loadingSubtreeItems.delete(itemId),
    mergeSubtreeItems: mergeItemsIntoCache,
    injectMockNode: (
      workspaceId: string,
      args: InjectMockNodeArgs = {},
    ): string | null => {
      const itemId = args.itemId ?? makeDebugItemId();
      const title = args.title?.trim() || 'Debug node';
      const description = args.description?.trim() || 'Frontend-only mock root node injected by __synthifyDebug.';
      const item = create(ItemSchema, {
        id: itemId,
        parentId: '',
        title,
        description,
        childIds: [],
        content: `<p>${description}</p>`,
      });
      const subtreeItem = create(SubtreeItemSchema, {
        item,
        hasChildren: false,
      });
      const previousRootNodeIds = workspaceRootNodeIds.get(workspaceId) ?? [];
      if (!previousRootNodeIds.includes(itemId)) {
        workspaceRootNodeIds.set(workspaceId, [...previousRootNodeIds, itemId]);
      }

      loadedSubtreeItems.add(itemId);
      mergeItemsIntoCache(workspaceId, [subtreeItem]);
      return itemId;
    },
    injectMockWorkspaceTree: (
      workspaceId: string,
      args: InjectMockWorkspaceTreeArgs = {},
    ): RefreshResult => {
      const items = buildMockWorkspaceTreeItems(workspaceId, args);
      initializedWorkspaces.add(workspaceId);
      return replaceWorkspaceTree(workspaceId, items);
    },
    debugSnapshot: (workspaceId: string): TreeStoreDebugSnapshot => {
      const treeItems = workspaceTreeItems.get(workspaceId) ?? new Map<string, SubtreeItem>();
      const workspaceItemIds = new Set(treeItems.keys());
      return {
        workspaceId,
        rootNodeIds: workspaceRootNodeIds.get(workspaceId) ?? [],
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
      workspaceRootNodeIds.clear();
      workspaceTreeItems.clear();
      loadedSubtreeItems.clear();
      loadingSubtreeItems.clear();
      fullyLoadedWorkspaces.clear();
      initializedWorkspaces.clear();
      newlyCreatedWorkspaces.clear();
    },
  };
}
