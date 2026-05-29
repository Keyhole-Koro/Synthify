'use client';

import { useEffect, useRef } from 'react';
import type { ExpansionMap } from '@keyhole-koro/paper-in-paper';
import { ROOT_ID, WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import type { Workspace } from '@/features/workspaces/api';
import type { TreeStore } from './useTreeStore';

interface UseExpansionWatcherArgs {
  expansionMap: ExpansionMap;
  workspaces: Workspace[];
  store: TreeStore;
  onLoadSubtree: (workspaceId: string, itemId: string) => void;
  onOpenWorkspace: (workspaceId: string) => void;
  onRebuildPaper: (workspaceId: string) => void;
  onExpansionMapChange: (next: ExpansionMap) => void;
}

// useExpansionWatcher reacts to expansion-map changes and to workspace list
// changes. It does not own tree data — it just triggers subtree loads when
// new items become open, and re-hydrates expanded workspaces on mount.
export function useExpansionWatcher({
  expansionMap,
  workspaces,
  store,
  onLoadSubtree,
  onOpenWorkspace,
  onRebuildPaper,
  onExpansionMapChange,
}: UseExpansionWatcherArgs) {
  const prevExpansionRef = useRef<ExpansionMap>(new Map());

  // Watch expansionMap changes and load subtrees for newly opened items.
  useEffect(() => {
    const prev = prevExpansionRef.current;
    if (expansionMap === prev) return;

    const newlyOpened: string[] = [];
    for (const [parentId, entry] of expansionMap) {
      const currentIds = entry?.openChildIds ?? [];
      const prevIds = prev.get(parentId)?.openChildIds ?? [];
      const prevSet = new Set(prevIds);
      for (const childId of currentIds) {
        if (!prevSet.has(childId)) newlyOpened.push(childId);
      }
    }
    prevExpansionRef.current = expansionMap;

    for (const itemId of newlyOpened) {
      if (!store.hasChildren(itemId)) continue;
      const workspaceId = store.getItemWorkspaceId(itemId);
      if (!workspaceId) continue;
      if (!store.getRootItemId(workspaceId)) continue;
      onLoadSubtree(workspaceId, itemId);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [expansionMap]);

  // Re-hydrate expanded workspaces on mount or when workspaces are loaded.
  useEffect(() => {
    if (workspaces.length === 0) return;

    for (const { workspaceId } of workspaces) {
      if (store.hasInitialized(workspaceId)) {
        onRebuildPaper(workspaceId);
      }
    }

    const rootOpenIds = expansionMap.get(ROOT_ID)?.openChildIds ?? [];
    if (!rootOpenIds.includes(WORKSPACES_ID)) return;

    const validWorkspaceIds = new Set([
      ...workspaces.map((workspace) => workspace.workspaceId),
      ...store.listNewlyCreatedIds(),
    ]);
    const workspacesOpenIds = expansionMap.get(WORKSPACES_ID)?.openChildIds ?? [];
    const validOpenWorkspaceIds = workspacesOpenIds.filter((workspaceId) => validWorkspaceIds.has(workspaceId));
    if (validOpenWorkspaceIds.length !== workspacesOpenIds.length) {
      const next = new Map(expansionMap);
      next.set(WORKSPACES_ID, { openChildIds: validOpenWorkspaceIds });
      onExpansionMapChange(next);
    }

    for (const workspaceId of validOpenWorkspaceIds) {
      if (!store.hasInitialized(workspaceId)) {
        onOpenWorkspace(workspaceId);
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaces]);

  return { prevExpansionRef };
}
