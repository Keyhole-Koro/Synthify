import type { DefaultOpenState, ExpansionMap } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import type { Workspace } from '@/features/workspaces/api';
import { authPaper, rootPaper, workspacesPaper } from '@/features/paperMap/staticPapers';

interface DefaultOpenStateOptions {
  user: AuthUser | null;
  workspaces: Workspace[];
}

const synthifyDefaultChildren = [
  'synthify:overview',
  'synthify:documents',
  'synthify:worker',
  'synthify:tree',
  'synthify:collaboration',
];

function synthifyExpansionMap(extraRootChildren: string[]): ExpansionMap {
  const map: ExpansionMap = new Map();
  map.set(rootPaper.id, { openChildIds: [authPaper.id, ...synthifyDefaultChildren, ...extraRootChildren] });
  map.set('synthify:overview', { openChildIds: ['synthify:overview:problem', 'synthify:overview:workflow'] });
  map.set('synthify:worker', { openChildIds: ['synthify:worker:tools', 'synthify:worker:lifecycle'] });
  map.set('synthify:tree', { openChildIds: ['synthify:tree:item', 'synthify:tree:links'] });
  return map;
}

export function mergeWithDefaultOpenState(
  persisted: DefaultOpenState | null,
  defaults: DefaultOpenState,
): DefaultOpenState {
  if (!persisted?.expansionMap) return defaults;

  const expansionMap: ExpansionMap = new Map(persisted.expansionMap);
  for (const [parentId, defaultEntry] of defaults.expansionMap ?? new Map()) {
    const current = expansionMap.get(parentId);
    if (!current) {
      expansionMap.set(parentId, defaultEntry);
      continue;
    }
    const openChildIds = Array.from(new Set([...defaultEntry.openChildIds, ...current.openChildIds]));
    expansionMap.set(parentId, { openChildIds });
  }

  return {
    expansionMap,
    focusedNodeId: persisted.focusedNodeId ?? defaults.focusedNodeId,
  };
}

export function computeDefaultOpenState(opts: DefaultOpenStateOptions): DefaultOpenState {
  if (opts.user && opts.workspaces.length > 0) {
    const map = synthifyExpansionMap([workspacesPaper.id]);
    return { expansionMap: map, focusedNodeId: workspacesPaper.id };
  }

  const map = synthifyExpansionMap([]);
  return { expansionMap: map, focusedNodeId: authPaper.id };
}
