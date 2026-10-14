import type { Paper } from '@keyhole-koro/paper-in-paper';
import type { SubtreeItem } from '@/features/tree/api';


/**
 * Converts a cached SubtreeItem into a renderable Paper object for the canvas,
 * adjusting its parent link to point to the workspace when it is a top-level root node.
 *
 * @param itemId - Unique node identifier.
 * @param treeItems - Map of all cached tree items in this workspace.
 * @param workspaceId - Workspace identifier acting as the root parent.
 * @returns The converted Paper object, or null if the item is missing.
 */
function buildProjectedPaper(
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

/**
 * Projected root node data used to render the cover report iframe in the workspace paper.
 */
export interface WorkspaceRootNodeProjection {
  /** Node identifier of the workspace root item. */
  id: string;
  /** HTML/Markdown content representing the synthesised workspace summary. */
  content: string;
  /** Optional custom CSS stylesheet injected into the report preview. */
  overrideCss?: string;
}

/**
 * Projects a workspace's loaded subtree nodes into an array of Paper definitions
 * consumed by the PaperCanvas layout engine.
 *
 * @param workspaceId - Target workspace identifier.
 * @param treeItems - Map of all loaded tree nodes.
 * @param rootNodeIds - Array of root item IDs found in the workspace tree.
 * @param buildWsPaper - Factory callback for generating the workspace parent paper.
 * @returns Array of all projected papers including the workspace container paper.
 */
export function projectWorkspacePapers(
  workspaceId: string,
  treeItems: Map<string, SubtreeItem>,
  rootNodeIds: string[],
  buildWsPaper: (
    workspaceId: string,
    childPapers: { id: string; title: string }[],
    rootNode?: WorkspaceRootNodeProjection,
  ) => Paper,
): Paper[] {
  const childPapers = rootNodeIds
    .map((id) => treeItems.get(id))
    .filter((item): item is SubtreeItem => item != null)
    .map((item) => ({ id: item.item!.id, title: item.item!.title }));

  // The workspace tree has a single root node whose content is the
  // workspace's cover report. Surface it so the workspace paper can render it
  // as a CSS-isolated iframe. If there are somehow multiple roots (transient
  // pre-merge state), the first is used.
  const rootItem = rootNodeIds
    .map((id) => treeItems.get(id)?.item)
    .find((it) => it != null);
  const rootNode: WorkspaceRootNodeProjection | undefined = rootItem
    ? { id: rootItem.id, content: rootItem.content, overrideCss: rootItem.overrideCss }
    : undefined;

  const projectedPapers: Paper[] = Array.from(treeItems.keys())
    .map((itemId) => buildProjectedPaper(itemId, treeItems, workspaceId))
    .filter((paper): paper is Paper => paper != null);

  return [...projectedPapers, buildWsPaper(workspaceId, childPapers, rootNode)];
}
