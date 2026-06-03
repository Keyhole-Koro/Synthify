import type { Paper } from '@keyhole-koro/paper-in-paper';
import type { SubtreeItem } from '@/features/tree/api';


export function buildProjectedPaper(
  itemId: string,
  treeItems: Map<string, SubtreeItem>,
  workspaceId: string,
): Paper | null {
  const it = treeItems.get(itemId)?.item;
  if (!it) return null;

  const projectedChildIds = it.childIds.filter((childId) => treeItems.has(childId));
  // The workspace itself is the tree root: nodes with no parent_id sit
  // directly under the workspace paper, so rewrite their parentId to the
  // workspaceId. Every other node keeps its parent node id.
  const projectedParentId = it.parentId ? it.parentId : workspaceId;

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
  treeItems: Map<string, SubtreeItem>,
  rootNodeIds: string[],
  buildWsPaper: (workspaceId: string, childPapers: { id: string; title: string }[]) => Paper,
): Paper[] {
  const childPapers = rootNodeIds
    .map((id) => treeItems.get(id))
    .filter((item): item is SubtreeItem => item != null)
    .map((item) => ({ id: item.item!.id, title: item.item!.title }));

  const projectedPapers: Paper[] = Array.from(treeItems.keys())
    .map((itemId) => buildProjectedPaper(itemId, treeItems, workspaceId))
    .filter((paper): paper is Paper => paper != null);

  return [...projectedPapers, buildWsPaper(workspaceId, childPapers)];
}
