import type { DefaultOpenState, ExpansionMap } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import type { Workspace } from '@/features/workspaces/api';

export const ROOT_ID = 'root';
export const AUTH_ID = 'auth';
export const WORKSPACES_ID = 'workspaces';

interface DefaultOpenStateOptions {
  user: AuthUser | null;
  workspaces: Workspace[];
}

const aboutDefaultChildren = [
  'synthify:about:promise',
  'synthify:about:getting-started',
  'synthify:about:fit',
  'synthify:overview',
  'synthify:documents',
  'synthify:worker',
  'synthify:tree',
  'synthify:operations',
];

function synthifyExpansionMap(extraRootChildren: string[]): ExpansionMap {
  const map: ExpansionMap = new Map();
  map.set(ROOT_ID, { openChildIds: [AUTH_ID, ...extraRootChildren] });
  map.set('synthify:about', { openChildIds: aboutDefaultChildren });
  map.set('synthify:overview', { openChildIds: ['synthify:overview:workflow', 'synthify:overview:reading'] });
  map.set('synthify:documents', { openChildIds: ['synthify:documents:intake', 'synthify:documents:chunks'] });
  map.set('synthify:worker', { openChildIds: ['synthify:worker:pipeline', 'synthify:worker:lifecycle'] });
  map.set('synthify:tree', { openChildIds: ['synthify:tree:item', 'synthify:tree:sources', 'synthify:tree:links'] });
  map.set('synthify:operations', { openChildIds: ['synthify:operations:workspace', 'synthify:operations:observability'] });
  return map;
}

export function computeDefaultOpenState(opts: DefaultOpenStateOptions): DefaultOpenState {
  if (opts.user) {
    const map = synthifyExpansionMap([WORKSPACES_ID]);
    const hasWorkspaces = opts.workspaces.length > 0;
    return {
      expansionMap: map,
      focusedNodeId: hasWorkspaces ? WORKSPACES_ID : AUTH_ID,
    };
  }

  const map = synthifyExpansionMap([]);
  return { expansionMap: map, focusedNodeId: AUTH_ID };
}
