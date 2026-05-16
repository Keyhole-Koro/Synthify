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

  const projectedChildIds = (it.childIds ?? []).filter(
    (childId) => childId !== workspaceRootItemId && treeItems.has(childId),
  );
  const projectedParentId =
    it.parentId === workspaceRootItemId ? workspaceId : (it.parentId ?? null);

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
  buildWsPaper: (workspaceId: string, childPapers: { id: string }[]) => Paper,
): Paper[] {
  const childPapers = documentRootIds
    .map((id) => treeItems.get(id))
    .filter((item): item is SubtreeItem => item != null)
    .map((item) => ({ id: item.item!.id }));

  const projectedPapers: Paper[] = Array.from(treeItems.keys())
    .map((itemId) => buildProjectedPaper(workspaceRootItemId, itemId, treeItems, workspaceId))
    .filter((paper): paper is Paper => paper != null);

  return [...projectedPapers, buildWsPaper(workspaceId, childPapers)];
}
