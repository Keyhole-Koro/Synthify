import type { ApiItem, SubtreeItem } from '@/features/tree/api';
import type { Workspace } from '@/features/workspaces/api';

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
}

// InjectMockWorkspaceTreeArgs builds a complete, frontend-only workspace tree
// (N root nodes, each with M child nodes) without touching the backend API.
// Used by __synthifyDebug to preview WorkspacePaper UI states.
export interface InjectMockWorkspaceTreeArgs {
  documentCount?: number;
  nodesPerDocument?: number;
  documentTitles?: string[];
}

export interface TreeStoreDebugSnapshot {
  workspaceId: string;
  rootNodeIds: string[];
  initialized: boolean;
  fullyLoaded: boolean;
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
  isFullyLoaded: (workspaceId: string) => boolean;
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
  injectMockWorkspaceTree: (workspaceId: string, args?: InjectMockWorkspaceTreeArgs) => RefreshResult;
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
  isFullyLoaded: (workspaceId: string) => boolean;
  getNewlyCreated: (workspaceId: string) => Workspace | undefined;
  listNewlyCreatedIds: () => string[];
  markInitialized: (workspaceId: string) => void;
  rememberNewlyCreated: (workspace: Workspace) => void;
  refreshWorkspaceTree: (workspaceId: string) => Promise<RefreshResult>;
  loadSubtree: (workspaceId: string, itemId: string, maxDepth?: number) => Promise<SubtreeItem[]>;
  injectMockNode: (workspaceId: string, args?: InjectMockNodeArgs) => string | null;
  injectMockWorkspaceTree: (workspaceId: string, args?: InjectMockWorkspaceTreeArgs) => RefreshResult;
  debugSnapshot: (workspaceId: string) => TreeStoreDebugSnapshot;
  reset: () => void;
}
