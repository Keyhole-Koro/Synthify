import { getSubtree, getTree, type SubtreeItem } from '@/features/tree/api';
import { log } from '@/lib/observability/log';
import type { RefreshResult, WorkspaceTreeCache } from './workspaceTreeTypes';

/**
 * Commands interface defining network fetch actions for workspace trees and subtrees.
 */
export interface WorkspaceTreeCommands {
  /** Asynchronously fetches and replaces the complete workspace tree. */
  refreshWorkspaceTree: (workspaceId: string) => Promise<RefreshResult>;
  /** Asynchronously fetches a subtree for a specific node up to maxDepth. */
  loadSubtree: (workspaceId: string, itemId: string, maxDepth?: number) => Promise<SubtreeItem[]>;
}

/**
 * Creates command handlers for refreshing trees and loading subtrees,
 * backed by the provided WorkspaceTreeCache.
 *
 * @param cache - The tree cache instance to update.
 * @returns Object providing `refreshWorkspaceTree` and `loadSubtree` command methods.
 */
export function createWorkspaceTreeCommands(cache: WorkspaceTreeCache): WorkspaceTreeCommands {
  return {
    refreshWorkspaceTree: async (workspaceId: string): Promise<RefreshResult> => {
      const tree = await getTree(workspaceId);
      return cache.replaceWorkspaceTree(workspaceId, tree.items);
    },
    loadSubtree: async (
      workspaceId: string,
      itemId: string,
      maxDepth = 1,
    ): Promise<SubtreeItem[]> => {
      if (cache.shouldSkipSubtreeLoad(workspaceId, itemId)) return [];
      cache.markSubtreeLoading(itemId);
      try {
        const items = await getSubtree(workspaceId, itemId, maxDepth);
        cache.mergeSubtreeItems(workspaceId, items);
        cache.markSubtreeLoaded(itemId);
        return items;
      } catch (err) {
        log.error('Failed to load subtree', { source: 'tree_load_subtree', workspaceId, itemId }, err);
        return [];
      } finally {
        cache.markSubtreeLoadFinished(itemId);
      }
    },
  };
}
