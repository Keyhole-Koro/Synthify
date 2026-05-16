import type { DefaultOpenState, ExpansionMap } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import type { Workspace } from '@/features/workspaces/api';
import { authPaper, rootPaper, workspacesPaper } from '@/features/paperMap/staticPapers';

interface DefaultOpenStateOptions {
  user: AuthUser | null;
  workspaces: Workspace[];
}

export function computeDefaultOpenState(opts: DefaultOpenStateOptions): DefaultOpenState {
  if (opts.user && opts.workspaces.length > 0) {
    const map: ExpansionMap = new Map();
    map.set(rootPaper.id, { openChildIds: [workspacesPaper.id] });
    return { expansionMap: map, focusedNodeId: workspacesPaper.id };
  }

  const map: ExpansionMap = new Map();
  map.set(rootPaper.id, { openChildIds: [authPaper.id] });
  return { expansionMap: map, focusedNodeId: authPaper.id };
}
