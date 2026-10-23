import { create } from '@bufbuild/protobuf';
import { ItemSchema, SubtreeItemSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import { findRootNodeIds } from '@/features/tree/buildTree';
import type { ApiItem, SubtreeItem } from '@/features/tree/api';
import type { Workspace } from '@/features/workspaces/api';
import { buildMockTree, type MockTreeSpec } from './mockTreeGenerator';
import type { InjectMockNodeArgs, InjectMockWorkspaceTreeArgs, InjectMockWorkspaceTreeResult, RefreshResult, TreeStoreDebugSnapshot, WorkspaceTreeCache } from './workspaceTreeTypes';

function makeDebugItemId() {
  return `debug_node_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

// toMockTreeSpec maps the console-facing args onto the generator's spec. The
// original documentCount / nodesPerDocument knobs described a flat one-root
// tree, so they translate to depth 1 with that many children — preserving the
// shape older console snippets expect. Any explicit MockTreeSpec knob wins.
function toMockTreeSpec(args: InjectMockWorkspaceTreeArgs): MockTreeSpec {
  const usesLegacyKnobs = args.documentCount != null || args.nodesPerDocument != null;
  const usesShapeKnobs = args.totalItems != null || args.depth != null || args.branching != null;

  if (usesLegacyKnobs && !usesShapeKnobs) {
    const childCount = Math.max(0, (args.documentCount ?? 2) * (args.nodesPerDocument ?? 3));
    return { ...args, depth: childCount > 0 ? 1 : 0, branching: Math.max(1, childCount), totalItems: childCount + 1 };
  }
  return args;
}

export function createWorkspaceTreeCache(): WorkspaceTreeCache {
  const itemWorkspace = new Map<string, string>();
  const itemHasChildren = new Map<string, boolean>();
  const workspaceRootNodeIds = new Map<string, string[]>();
  const workspaceTreeItems = new Map<string, Map<string, SubtreeItem>>();
  const loadedSubtreeItems = new Set<string>();
  const loadingSubtreeItems = new Set<string>();
  const outlineLoadedWorkspaces = new Set<string>();
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
    const previousItemIds = Array.from(workspaceTreeItems.get(workspaceId)?.keys() ?? []);
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

    outlineLoadedWorkspaces.add(workspaceId);

    // GetTree returns an outline: bodies arrive only for root nodes. So only
    // the roots count as body-loaded — every other item still needs a
    // GetSubtree when its paper opens. Marking them all loaded (as this did
    // while GetTree carried every body) would suppress those fetches and leave
    // the papers showing the description fallback forever.
    //
    // Previously-loaded bodies are dropped along with the items they belonged
    // to: this replaces the workspace's item map, so a stale "loaded" mark
    // would claim a body that is no longer in the cache.
    for (const id of previousItemIds) {
      loadedSubtreeItems.delete(id);
      loadingSubtreeItems.delete(id);
    }
    for (const id of rootNodeIds) {
      loadedSubtreeItems.add(id);
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
    isOutlineLoaded: (workspaceId) => outlineLoadedWorkspaces.has(workspaceId),
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
    // The workspace having its outline is no longer a reason to skip: the
    // outline carries no bodies. Only an in-flight or completed fetch for this
    // specific item is.
    shouldSkipSubtreeLoad: (_workspaceId, itemId) =>
      loadingSubtreeItems.has(itemId) || loadedSubtreeItems.has(itemId),
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
      const description = args.description?.trim() || 'Frontend-only mock node injected by __synthifyDebug.';
      const parentId = args.parentId?.trim() ?? '';
      const item = create(ItemSchema, {
        id: itemId,
        parentId,
        title,
        description,
        childIds: [],
        content: args.content ?? `<p>${description}</p>`,
        overrideCss: args.overrideCss ?? '',
      });
      const subtreeItem = create(SubtreeItemSchema, {
        item,
        hasChildren: false,
      });

      if (parentId) {
        // Hang under an existing node: append to the parent's childIds so the
        // projection surfaces it, and mark the parent as having children.
        const workspaceItems = workspaceTreeItems.get(workspaceId);
        const parent = workspaceItems?.get(parentId);
        if (parent?.item && !parent.item.childIds.includes(itemId)) {
          parent.item.childIds = [...parent.item.childIds, itemId];
        }
        itemHasChildren.set(parentId, true);
      } else {
        // Top-level node: it becomes one of the workspace paper's direct children.
        const previousRootNodeIds = workspaceRootNodeIds.get(workspaceId) ?? [];
        if (!previousRootNodeIds.includes(itemId)) {
          workspaceRootNodeIds.set(workspaceId, [...previousRootNodeIds, itemId]);
        }
      }

      loadedSubtreeItems.add(itemId);
      mergeItemsIntoCache(workspaceId, [subtreeItem]);
      return itemId;
    },
    injectMockWorkspaceTree: (
      workspaceId: string,
      args: InjectMockWorkspaceTreeArgs = {},
    ): InjectMockWorkspaceTreeResult => {
      const { items, resolved } = buildMockTree(workspaceId, toMockTreeSpec(args));
      initializedWorkspaces.add(workspaceId);
      const result = replaceWorkspaceTree(workspaceId, items);
      // A mock tree is complete: every node already carries its body, unlike a
      // GetTree outline. Mark them all body-loaded so opening a paper does not
      // fire a GetSubtree for a workspace the backend has never heard of.
      for (const item of items) {
        loadedSubtreeItems.add(item.id);
      }
      return { ...result, resolved };
    },
    debugSnapshot: (workspaceId: string): TreeStoreDebugSnapshot => {
      const treeItems = workspaceTreeItems.get(workspaceId) ?? new Map<string, SubtreeItem>();
      const workspaceItemIds = new Set(treeItems.keys());
      return {
        workspaceId,
        rootNodeIds: workspaceRootNodeIds.get(workspaceId) ?? [],
        initialized: initializedWorkspaces.has(workspaceId),
        outlineLoaded: outlineLoadedWorkspaces.has(workspaceId),
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
      outlineLoadedWorkspaces.clear();
      initializedWorkspaces.clear();
      newlyCreatedWorkspaces.clear();
    },
  };
}
