import type { ApiItem, SubtreeItem } from '@/features/tree/api';
import type { Workspace } from '@/features/workspaces/api';
import type { MockTreeSpec, ResolvedMockTreeSpec } from './mockTreeGenerator';

export interface RefreshResult {
  // rootNodeIds are the workspace's top-level nodes (parent_id IS NULL). The
  // workspace itself is the tree root, so these are the workspace paper's
  // direct children.
  rootNodeIds: string[];
  newRootNodeIds: string[];
}

export interface InjectMockNodeArgs {
  itemId?: string;
  title?: string;
  description?: string;
  // content overrides the node's rich HTML (rendered as the cover report iframe
  // when this node is the workspace root). Defaults to a <p> of the description.
  content?: string;
  // overrideCss is the node's per-item CSS, isolated inside the content iframe.
  overrideCss?: string;
  // parentId hangs the node under an existing node instead of at top level.
  // Empty / omitted makes it a top-level node (parent_id IS NULL).
  parentId?: string;
}

// InjectMockWorkspaceTreeArgs builds a complete, frontend-only workspace tree
// without touching the backend API. Used by __synthifyDebug to preview
// WorkspacePaper UI states and by the load-test harness to give the client an
// arbitrary number of items. The shape knobs (totalItems / depth / branching /
// contentBytes / seed) come from MockTreeSpec; see mockTreeGenerator.ts.
export interface InjectMockWorkspaceTreeArgs extends MockTreeSpec {
  // documentCount / nodesPerDocument are the original console-facing knobs.
  // They still produce the old flat shape (one root, documentCount ×
  // nodesPerDocument children) when no MockTreeSpec knob is given.
  documentCount?: number;
  nodesPerDocument?: number;
  documentTitles?: string[];
}

// InjectMockWorkspaceTreeResult reports the tree that was actually built, so a
// measurement is never ambiguous about its input.
export interface InjectMockWorkspaceTreeResult extends RefreshResult {
  resolved: ResolvedMockTreeSpec;
}

export interface TreeStoreDebugSnapshot {
  workspaceId: string;
  rootNodeIds: string[];
  initialized: boolean;
  // outlineLoaded: the workspace tree structure has been fetched. Node
  // bodies are separate — see loadedItemIds.
  outlineLoaded: boolean;
  itemCount: number;
  treeItems: SubtreeItem[];
  loadedItemIds: string[];
  loadingItemIds: string[];
}

export interface WorkspaceTreeCache {
  getRootNodeIds: (workspaceId: string) => string[];
  getTreeItems: (workspaceId: string) => Map<string, SubtreeItem>;
  getItemWorkspaceId: (itemId: string) => string | undefined;
  hasInitialized: (workspaceId: string) => boolean;
  isLoaded: (itemId: string) => boolean;
  isLoading: (itemId: string) => boolean;
  hasChildren: (itemId: string) => boolean;
  isOutlineLoaded: (workspaceId: string) => boolean;
  getNewlyCreated: (workspaceId: string) => Workspace | undefined;
  listNewlyCreatedIds: () => string[];
  markInitialized: (workspaceId: string) => void;
  rememberNewlyCreated: (workspace: Workspace) => void;
  pruneNewlyCreated: (workspaces: Workspace[]) => void;
  replaceWorkspaceTree: (workspaceId: string, items: ApiItem[]) => RefreshResult;
  shouldSkipSubtreeLoad: (workspaceId: string, itemId: string) => boolean;
  markSubtreeLoading: (itemId: string) => void;
  markSubtreeLoaded: (itemId: string) => void;
  markSubtreeLoadFinished: (itemId: string) => void;
  mergeSubtreeItems: (workspaceId: string, items: SubtreeItem[]) => void;
  injectMockNode: (workspaceId: string, args?: InjectMockNodeArgs) => string | null;
  injectMockWorkspaceTree: (workspaceId: string, args?: InjectMockWorkspaceTreeArgs) => InjectMockWorkspaceTreeResult;
  debugSnapshot: (workspaceId: string) => TreeStoreDebugSnapshot;
  reset: () => void;
}

export interface TreeStore {
  getRootNodeIds: (workspaceId: string) => string[];
  getTreeItems: (workspaceId: string) => Map<string, SubtreeItem>;
  getItemWorkspaceId: (itemId: string) => string | undefined;
  hasInitialized: (workspaceId: string) => boolean;
  isLoaded: (itemId: string) => boolean;
  isLoading: (itemId: string) => boolean;
  hasChildren: (itemId: string) => boolean;
  isOutlineLoaded: (workspaceId: string) => boolean;
  getNewlyCreated: (workspaceId: string) => Workspace | undefined;
  listNewlyCreatedIds: () => string[];
  markInitialized: (workspaceId: string) => void;
  rememberNewlyCreated: (workspace: Workspace) => void;
  refreshWorkspaceTree: (workspaceId: string) => Promise<RefreshResult>;
  loadSubtree: (workspaceId: string, itemId: string, maxDepth?: number) => Promise<SubtreeItem[]>;
  injectMockNode: (workspaceId: string, args?: InjectMockNodeArgs) => string | null;
  injectMockWorkspaceTree: (workspaceId: string, args?: InjectMockWorkspaceTreeArgs) => InjectMockWorkspaceTreeResult;
  debugSnapshot: (workspaceId: string) => TreeStoreDebugSnapshot;
  reset: () => void;
}
