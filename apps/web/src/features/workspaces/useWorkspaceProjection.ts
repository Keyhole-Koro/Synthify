import type { Paper } from '@keyhole-koro/paper-in-paper';
import type { SubtreeItem } from '@/features/tree/api';


export function buildProjectedPaper(
  workspaceRootItemId: string,
  itemId: string,
  treeItems: Map<string, SubtreeItem>,
  workspaceId: string,
): Paper | null {
  const it = treeItems.get(itemId)?.item;
  if (!it || it.id === workspaceRootItemId) return null;

  const projectedChildIds = it.childIds.filter(
    (childId) => childId !== workspaceRootItemId && treeItems.has(childId),
  );
  // The workspace_root paper is hidden from the projection; rewrite items
  // whose parent is the workspace_root so they appear under the workspace
  // paper itself. Every other item has a non-empty parentId by schema
  // invariant (only workspace_root has parentId IS NULL).
  const projectedParentId =
    it.parentId === workspaceRootItemId ? workspaceId : it.parentId;

  return {
    ...it,
    content: it.content || `<p>${it.description}</p>`,
    hue: 220,
    parentId: projectedParentId,
    childIds: projectedChildIds,
  } satisfies Paper;
}

export function projectWorkspacePapers(
  workspaceId: string,
  workspaceRootItemId: string,
  treeItems: Map<string, SubtreeItem>,
  documentRootIds: string[],
  buildWsPaper: (workspaceId: string, childPapers: { id: string; title: string }[]) => Paper,
): Paper[] {
  const childPapers = documentRootIds
    .map((id) => treeItems.get(id))
    .filter((item): item is SubtreeItem => item != null)
    .map((item) => ({ id: item.item!.id, title: item.item!.title }));

  const projectedPapers: Paper[] = Array.from(treeItems.keys())
    .map((itemId) => buildProjectedPaper(workspaceRootItemId, itemId, treeItems, workspaceId))
    .filter((paper): paper is Paper => paper != null);

  return [...projectedPapers, buildWsPaper(workspaceId, childPapers)];
}
