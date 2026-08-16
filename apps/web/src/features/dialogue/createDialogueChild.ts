import type { PaperId, PaperMap } from '@keyhole-koro/paper-in-paper';

/**
 * Walks up to the workspace that owns `paperId`.
 *
 * Workspace papers use the workspace id as their paper id, so the owning
 * workspace is the nearest ancestor whose id appears in `workspaceIds`.
 * Resolving it by walking rather than threading an "active workspace" through
 * the controller keeps this working on the landing canvas, where several
 * workspaces are on screen at once and there is no single active one.
 */
export function findOwningWorkspaceId(
  paperMap: PaperMap,
  paperId: PaperId,
  workspaceIds: ReadonlySet<string>,
): string | null {
  let current: PaperId | null = paperId;
  const seen = new Set<PaperId>();

  while (current && !seen.has(current)) {
    seen.add(current);
    if (workspaceIds.has(current)) return current;
    current = paperMap.get(current)?.parentId ?? null;
  }
  return null;
}
