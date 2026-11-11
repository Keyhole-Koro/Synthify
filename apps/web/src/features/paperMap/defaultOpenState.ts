import type { DefaultOpenState, ExpansionMap } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import type { Workspace } from '@/features/workspaces/api';

/** Identifier for the top-level root paper container. */
export const ROOT_ID = 'root';
/** Identifier for the authentication / profile paper node. */
export const AUTH_ID = 'auth';
/** Identifier for the workspace listing paper node. */
export const WORKSPACES_ID = 'workspaces';

/**
 * Options used to calculate the default initial open state of the paper canvas.
 */
interface DefaultOpenStateOptions {
  /** The currently authenticated user, or null if anonymous. */
  user: AuthUser | null;
  /** List of workspaces accessible to the user. */
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

/**
 * Constructs the canonical expansion map containing default opened children
 * for informational about/documentation paper nodes.
 *
 * @param extraRootChildren - Additional paper IDs to open directly under the root paper.
 * @returns The populated expansion map for paper-in-paper canvas initialization.
 */
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

/**
 * Computes the initial set of opened papers and the focused node ID
 * according to user login state and workspace presence.
 *
 * @param opts - User session and workspace details.
 * @returns The resolved default open state object for canvas initial mounting.
 */
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
