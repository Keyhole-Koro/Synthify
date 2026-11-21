import type { ApiItem, SubtreeItem } from '@/features/tree/api';
import type { Workspace } from '@/features/workspaces/api';

/**
 * Result returned after refreshing a workspace tree from the backend.
 */
export interface RefreshResult {
  /** Top-level root node IDs in the workspace tree. */
  rootNodeIds: string[];
  /** Subset of root node IDs newly discovered in this refresh cycle. */
  newRootNodeIds: string[];
}

/**
 * Options for injecting a synthetic mock tree node for offline testing.
 */
export interface InjectMockNodeArgs {
  /** Explicit node ID to assign, or generated automatically if omitted. */
  itemId?: string;
  /** Title for the mock node. */
  title?: string;
  /** Short summary description for the mock node. */
  description?: string;
  /** HTML content body rendered in the node paper or cover iframe. */
  content?: string;
  /** Custom CSS styling rules for the node. */
  overrideCss?: string;
  /** Parent node ID, or omitted if placing directly at the workspace root level. */
  parentId?: string;
}

/**
 * Options for generating a complete mock tree topology offline.
 */
export interface InjectMockWorkspaceTreeArgs {
  /** Number of root document nodes to generate. */
  documentCount?: number;
  /** Number of descendant detail nodes per root document. */
  nodesPerDocument?: number;
  /** Custom titles for generated document root nodes. */
  documentTitles?: string[];
  /** Custom HTML content for the primary cover report. */
  rootContent?: string;
  /** Custom CSS styling rules for the primary cover report. */
  rootOverrideCss?: string;
}

/**
 * Diagnostic debug snapshot capturing the internal memory state of a workspace tree.
 */
export interface TreeStoreDebugSnapshot {
  /** Target workspace identifier. */
  workspaceId: string;
  /** Top-level root node IDs. */
  rootNodeIds: string[];
  /** Whether the workspace has been initialized in cache. */
  initialized: boolean;
  /** Whether the full initial tree has finished loading. */
  fullyLoaded: boolean;
  /** Total count of items currently stored in cache. */
  itemCount: number;
  /** List of all stored SubtreeItem records. */
  treeItems: SubtreeItem[];
  /** IDs of items whose subtrees have loaded. */
  loadedItemIds: string[];
  /** IDs of items whose subtrees are currently loading. */
  loadingItemIds: string[];
}

/**
 * Low-level in-memory cache and mutation methods for managing workspace tree nodes.
 */
export interface WorkspaceTreeCache {
  /** Returns the list of top-level root node IDs for a workspace. */
  getRootNodeIds: (workspaceId: string) => string[];
  /** Returns the map of all stored subtree items for a workspace. */
  getTreeItems: (workspaceId: string) => Map<string, SubtreeItem>;
  /** Finds the workspace ID owning a specific item ID. */
  getItemWorkspaceId: (itemId: string) => string | undefined;
  /** Returns whether a workspace has been initialized. */
  hasInitialized: (workspaceId: string) => boolean;
  /** Returns whether a node's children have finished loading. */
  isLoaded: (itemId: string) => boolean;
  /** Returns whether a node's children are actively being fetched. */
  isLoading: (itemId: string) => boolean;
  /** Returns whether a node has child items. */
  hasChildren: (itemId: string) => boolean;
  /** Returns whether the workspace's entire root tree is loaded. */
  isFullyLoaded: (workspaceId: string) => boolean;
  /** Retrieves an optimistic newly-created workspace entity by ID. */
  getNewlyCreated: (workspaceId: string) => Workspace | undefined;
  /** Returns all tracked newly-created workspace IDs. */
  listNewlyCreatedIds: () => string[];
  /** Marks a workspace as initialized in cache. */
  markInitialized: (workspaceId: string) => void;
  /** Caches an optimistic newly-created workspace entity. */
  rememberNewlyCreated: (workspace: Workspace) => void;
  /** Removes optimistic workspace entries once they appear in the authoritative list. */
  pruneNewlyCreated: (workspaces: Workspace[]) => void;
  /** Replaces the cached tree for a workspace with fresh API items. */
  replaceWorkspaceTree: (workspaceId: string, items: ApiItem[]) => RefreshResult;
  /** Checks if a subtree fetch can be skipped due to existing data or in-flight requests. */
  shouldSkipSubtreeLoad: (workspaceId: string, itemId: string) => boolean;
  /** Marks a node as actively loading its subtree. */
  markSubtreeLoading: (itemId: string) => void;
  /** Marks a node as having loaded its subtree. */
  markSubtreeLoaded: (itemId: string) => void;
  /** Clears the in-flight loading flag for a node. */
  markSubtreeLoadFinished: (itemId: string) => void;
  /** Merges fetched subtree items into the workspace cache. */
  mergeSubtreeItems: (workspaceId: string, items: SubtreeItem[]) => void;
  /** Injects a synthetic mock node for testing. */
  injectMockNode: (workspaceId: string, args?: InjectMockNodeArgs) => string | null;
  /** Injects an entire mock tree topology for testing. */
  injectMockWorkspaceTree: (workspaceId: string, args?: InjectMockWorkspaceTreeArgs) => RefreshResult;
  /** Produces a diagnostic debug snapshot of the cache state. */
  debugSnapshot: (workspaceId: string) => TreeStoreDebugSnapshot;
  /** Clears all cached tree and workspace data. */
  reset: () => void;
}

/**
 * Public facade interface combining cached tree data access with asynchronous network commands.
 */
export interface TreeStore {
  /** Returns top-level root node IDs for a workspace. */
  getRootNodeIds: (workspaceId: string) => string[];
  /** Returns map of stored subtree items for a workspace. */
  getTreeItems: (workspaceId: string) => Map<string, SubtreeItem>;
  /** Finds workspace ID owning a specific item. */
  getItemWorkspaceId: (itemId: string) => string | undefined;
  /** Checks if workspace has been initialized in store. */
  hasInitialized: (workspaceId: string) => boolean;
  /** Checks if item's subtree is loaded. */
  isLoaded: (itemId: string) => boolean;
  /** Checks if item's subtree is actively loading. */
  isLoading: (itemId: string) => boolean;
  /** Checks if item has child nodes. */
  hasChildren: (itemId: string) => boolean;
  /** Checks if workspace is fully loaded. */
  isFullyLoaded: (workspaceId: string) => boolean;
  /** Retrieves an optimistic newly-created workspace. */
  getNewlyCreated: (workspaceId: string) => Workspace | undefined;
  /** Lists all newly-created workspace IDs. */
  listNewlyCreatedIds: () => string[];
  /** Marks a workspace as initialized. */
  markInitialized: (workspaceId: string) => void;
  /** Remembers an optimistic newly-created workspace. */
  rememberNewlyCreated: (workspace: Workspace) => void;
  /** Asynchronously fetches and updates the entire tree for a workspace. */
  refreshWorkspaceTree: (workspaceId: string) => Promise<RefreshResult>;
  /** Asynchronously fetches a node's descendants up to a specified depth. */
  loadSubtree: (workspaceId: string, itemId: string, maxDepth?: number) => Promise<SubtreeItem[]>;
  /** Injects a single synthetic mock node. */
  injectMockNode: (workspaceId: string, args?: InjectMockNodeArgs) => string | null;
  /** Injects a complete mock tree topology. */
  injectMockWorkspaceTree: (workspaceId: string, args?: InjectMockWorkspaceTreeArgs) => RefreshResult;
  /** Returns a diagnostic debug snapshot. */
  debugSnapshot: (workspaceId: string) => TreeStoreDebugSnapshot;
  /** Resets the entire store. */
  reset: () => void;
}
