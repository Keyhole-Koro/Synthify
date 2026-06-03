import type { SubtreeItem } from '@/features/tree/api';
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

export interface TreeStoreDebugSnapshot {
  workspaceId: string;
  workspaceRootItemId?: string;
  documentRootIds: string[];
  initialized: boolean;
  fullyLoaded: boolean;
  itemCount: number;
  treeItems: SubtreeItem[];
  loadedItemIds: string[];
  loadingItemIds: string[];
}

export interface WorkspaceTreeCache {
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
  markInitialized: (workspaceId: string) => void;
  rememberNewlyCreated: (workspace: Workspace) => void;
  pruneNewlyCreated: (workspaces: Workspace[]) => void;
  replaceWorkspaceTree: (workspaceId: string, items: import('@/features/tree/api').ApiItem[]) => RefreshResult;
  shouldSkipSubtreeLoad: (workspaceId: string, itemId: string) => boolean;
  markSubtreeLoading: (itemId: string) => void;
  markSubtreeLoaded: (itemId: string) => void;
  markSubtreeLoadFinished: (itemId: string) => void;
  mergeSubtreeItems: (workspaceId: string, items: SubtreeItem[]) => void;
  mergeDocumentRootItems: (workspaceId: string, createdDocumentRootItemId: string, items: SubtreeItem[]) => MergeDocumentRootResult | null;
  debugSnapshot: (workspaceId: string) => TreeStoreDebugSnapshot;
  reset: () => void;
}

export interface TreeStore {
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
  markInitialized: (workspaceId: string) => void;
  rememberNewlyCreated: (workspace: Workspace) => void;
  refreshWorkspaceTree: (workspaceId: string) => Promise<RefreshResult>;
  loadSubtree: (workspaceId: string, itemId: string, maxDepth?: number) => Promise<SubtreeItem[]>;
  mergeDocumentRoot: (
    workspaceId: string,
    createdDocumentRootItemId: string,
  ) => Promise<MergeDocumentRootResult | null>;
  debugSnapshot: (workspaceId: string) => TreeStoreDebugSnapshot;
  reset: () => void;
}
