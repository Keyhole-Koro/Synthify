import { getSubtree, getTree, type SubtreeItem } from '@/features/tree/api';
import { log } from '@/lib/observability/log';
import type { MergeDocumentRootResult, RefreshResult, WorkspaceTreeCache } from './workspaceTreeTypes';

export interface WorkspaceTreeCommands {
  refreshWorkspaceTree: (workspaceId: string) => Promise<RefreshResult>;
  loadSubtree: (workspaceId: string, itemId: string, maxDepth?: number) => Promise<SubtreeItem[]>;
  mergeDocumentRoot: (
    workspaceId: string,
    createdDocumentRootItemId: string,
  ) => Promise<MergeDocumentRootResult | null>;
}

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
    mergeDocumentRoot: async (
      workspaceId: string,
      createdDocumentRootItemId: string,
    ): Promise<MergeDocumentRootResult | null> => {
      if (!cache.getRootItemId(workspaceId)) return null;
      const items = await getSubtree(workspaceId, createdDocumentRootItemId, 5);
      return cache.mergeDocumentRootItems(workspaceId, createdDocumentRootItemId, items);
    },
  };
}
