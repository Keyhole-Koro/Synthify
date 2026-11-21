
import { useEffect, useRef } from 'react';
import type { ExpansionMap } from '@keyhole-koro/paper-in-paper';
import { ROOT_ID, WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import type { Workspace } from '@/features/workspaces/api';
import type { TreeStore } from './workspaceTreeTypes';

/**
 * Arguments for the {@link useExpansionWatcher} hook.
 */
interface UseExpansionWatcherArgs {
  /** Current map of opened paper node IDs. */
  expansionMap: ExpansionMap;
  /** List of workspaces accessible to the user. */
  workspaces: Workspace[];
  /** Tree store instance containing cached nodes. */
  store: TreeStore;
  /** Callback fired to load a newly revealed node's subtree. */
  onLoadSubtree: (workspaceId: string, itemId: string) => void;
  /** Callback fired to open and initialize a workspace on canvas. */
  onOpenWorkspace: (workspaceId: string) => void;
  /** Callback fired to re-project papers for an initialized workspace. */
  onRebuildPaper: (workspaceId: string) => void;
  /** Callback fired when pruned or updated expansion maps are produced. */
  onExpansionMapChange: (next: ExpansionMap) => void;
}

/**
 * Observes changes to the paper canvas expansion map and user workspaces list,
 * automatically triggering subtree fetching for newly exposed nodes and hydrating
 * workspaces that are opened in the persisted canvas state.
 *
 * @param args - Watcher configuration and event callbacks.
 * @returns Ref containing the previous expansion map state.
 */
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
      if (!store.isFullyLoaded(workspaceId)) continue;
      onLoadSubtree(workspaceId, itemId);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [expansionMap]);

  // Rebuild papers for already-initialized workspaces when the list changes
  // (e.g. workspace rename). Keyed on [workspaces] only — rebuilding produces a
  // fresh paper array every call, so running it on every expansionMap change
  // would churn workspacePaperGroups and trigger needless re-renders.
  useEffect(() => {
    if (workspaces.length === 0) return;
    for (const { workspaceId } of workspaces) {
      if (store.hasInitialized(workspaceId)) {
        onRebuildPaper(workspaceId);
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaces]);

  // Fetch the tree for workspaces that are open in the persisted expansion map.
  //
  // Depends on expansionMap too, not just workspaces: on reload the workspace
  // list often resolves BEFORE the persisted open-state is restored
  // (usePersistentPaperOpenState restores expansionMap from localStorage in a
  // separate effect, and may block on Firebase auth). If this ran only on
  // [workspaces], it would see an empty expansionMap, find no open workspaces,
  // and never fetch their trees — the "expanded but not fetched" bug.
  // Re-running when expansionMap arrives lets us pick up the restored open
  // workspaces. onOpenWorkspace is guarded by hasInitialized below, so an
  // already-fetched workspace is not re-fetched on later expansionMap changes.
  useEffect(() => {
    if (workspaces.length === 0) return;

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
  }, [workspaces, expansionMap]);

  return { prevExpansionRef };
}
