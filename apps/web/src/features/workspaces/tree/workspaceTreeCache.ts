import { create } from '@bufbuild/protobuf';
import { ItemSchema, SubtreeItemSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import { findRootNodeIds } from '@/features/tree/buildTree';
import type { ApiItem, SubtreeItem } from '@/features/tree/api';
import type { Workspace } from '@/features/workspaces/api';
import type { InjectMockNodeArgs, InjectMockWorkspaceTreeArgs, RefreshResult, TreeStoreDebugSnapshot, WorkspaceTreeCache } from './workspaceTreeTypes';

function makeDebugItemId() {
  return `debug_node_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

// SAMPLE_ROOT_OVERRIDE_CSS is injected into the root content iframe so the CSS
// isolation path is exercised: these selectors would clash with the host page
// if they leaked, but stay contained inside the sandboxed iframe.
const SAMPLE_ROOT_OVERRIDE_CSS = `
.hero-block { background: linear-gradient(135deg, #eef2ff, #fdf2f8); border-radius: 16px; padding: 18px 20px; }
h1 { color: #4338ca; }
.metric { display: inline-block; margin-right: 16px; font-weight: 700; color: #be185d; }
`.trim();

// buildSampleRootContent renders a rich cover report that embeds child node ids
// as data-paper-id links directly in the content. WorkspaceRootContent forwards
// in-iframe clicks on these links to the host, which opens the child paper — so
// the embedded links are live navigation, exercising that path from the console.
function buildSampleRootContent(childRefs: { id: string; title: string }[]): string {
  const links = childRefs
    .map((c) => `<a data-paper-id="${c.id}">${c.title}</a>`)
    .join(' ');
  return `
<div class="hero-block">
  <h1>統合知識レポート（モック）</h1>
  <p class="lede">__synthifyDebug が生成したフロントエンド専用のダミー root content です。リッチ HTML + override_css が iframe で CSS 隔離描画されます。</p>
  <p><span class="metric">出典 3</span><span class="metric">ノード ${childRefs.length}</span></p>
</div>
<h2>概要</h2>
<p>このワークスペースの知識ツリーのトップ概要。下のリンクから配下のノードを開けます。</p>
<table><thead><tr><th>項目</th><th>値</th></tr></thead><tbody>
<tr><td>モデル</td><td>node 直属</td></tr><tr><td>root</td><td>単一</td></tr></tbody></table>
<h2>子ノード（クリックで開く）</h2>
<p>${links || '（子ノードなし）'}</p>
`.trim();
}

// buildMockWorkspaceTreeItems assembles a complete ApiItem[] for a single-root
// workspace tree entirely on the frontend, mirroring the worker's node-direct
// model: one root node (parent_id = '') carrying the cover-report content, with
// M child knowledge nodes hanging off it. The shape mirrors what the backend
// returns so it can flow through replaceWorkspaceTree unchanged.
function buildMockWorkspaceTreeItems(
  workspaceId: string,
  args: InjectMockWorkspaceTreeArgs,
): ApiItem[] {
  // documentCount is kept for backward compatibility but now controls how many
  // child knowledge nodes hang under the single root (each "document" produced
  // one root node group in the old model; now they are all children of one root).
  const childCount = Math.max(0, (args.documentCount ?? 2) * (args.nodesPerDocument ?? 3));
  const titles = args.documentTitles ?? [];

  const rootId = `debug_root_${workspaceId}`;
  const items: ApiItem[] = [];
  const childRefs: { id: string; title: string }[] = [];

  for (let n = 0; n < childCount; n++) {
    const nodeId = `${rootId}_node_${n}`;
    const title = titles[n]?.trim() || `知識ノード ${n + 1}`;
    childRefs.push({ id: nodeId, title });
    items.push(create(ItemSchema, {
      id: nodeId,
      parentId: rootId,
      title,
      description: `Mock child node ${n + 1}`,
      content: `<p>${title} の本文。__synthifyDebug が生成したフロントエンド専用のダミーです。</p>`,
      level: 1,
      childIds: [],
    }));
  }

  items.push(create(ItemSchema, {
    id: rootId,
    parentId: '',
    title: titles[0]?.trim() || 'モック ワークスペース',
    description: 'Mock single root node',
    content: args.rootContent ?? buildSampleRootContent(childRefs),
    overrideCss: args.rootOverrideCss ?? SAMPLE_ROOT_OVERRIDE_CSS,
    level: 0,
    childIds: childRefs.map((c) => c.id),
  }));

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
