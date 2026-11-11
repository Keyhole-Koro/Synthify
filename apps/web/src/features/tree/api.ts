import { createRPCClient, createPublicRPCClient } from '@/lib/connect';
import { TreeService } from '@/gen/proto/synthify/app/v1/tree_pb';
import { ItemService } from '@/gen/proto/synthify/app/v1/item_pb';
import { JobService } from '@/gen/proto/synthify/app/v1/job_pb';
import { TreeProjectionScope, type SubtreeItem } from '@/gen/proto/synthify/app/v1/tree_types_pb';

export type { Item as ApiItem, SubtreeItem } from '@/gen/proto/synthify/app/v1/tree_types_pb';
export type { EntityRef, TreeEntityDetail } from '@/gen/proto/synthify/app/v1/item_pb';
export { TreeProjectionScope } from '@/gen/proto/synthify/app/v1/tree_types_pb';

const treeClient = createRPCClient(TreeService);
const itemClient = createRPCClient(ItemService);
const jobClient = createRPCClient(JobService);

// For public link viewers: authenticated with X-Synthify-Share-Token header instead of Bearer token.
const publicTreeClient = createPublicRPCClient(TreeService);

/**
 * Fetches the entire knowledge tree structure for a workspace.
 *
 * @param workspaceId - Workspace identifier.
 * @returns Complete Tree object.
 */
export async function getTree(workspaceId: string) {
  const res = await treeClient.getTree({ workspaceId });
  return res.tree!;
}

/**
 * Fetches the knowledge tree via a public share token without requiring user authentication.
 *
 * @param workspaceId - Workspace identifier.
 * @param shareToken - Public share link access token.
 * @returns Complete Tree object.
 */
export async function getTreeViaShareToken(workspaceId: string, shareToken: string) {
  const res = await publicTreeClient.getTree(
    { workspaceId },
    { headers: { 'X-Synthify-Share-Token': shareToken } },
  );
  return res.tree!;
}

/**
 * Retrieves detailed entity information for a specific node in the tree.
 *
 * @param targetRef - Target entity reference containing workspace ID, projection scope, and entity ID.
 * @param resolveAliases - Whether to resolve alias references.
 * @returns Detailed TreeEntityDetail.
 */
export async function getTreeEntityDetail(
  targetRef: { workspaceId: string; scope: TreeProjectionScope; id: string; documentId?: string },
  resolveAliases = false,
) {
  const res = await itemClient.getTreeEntityDetail({
    targetRef,
    resolveAliases,
  });
  return res.detail!;
}

/**
 * Retrieves a subtree rooted at a specific item down to a maximum traversal depth.
 *
 * @param workspaceId - Workspace identifier.
 * @param itemId - Root item of the subtree.
 * @param maxDepth - Maximum descent depth (defaults to 3).
 * @returns Array of SubtreeItem objects.
 */
export async function getSubtree(
  workspaceId: string,
  itemId: string,
  maxDepth = 3,
): Promise<SubtreeItem[]> {
  const res = await treeClient.getSubtree({ workspaceId, itemId, maxDepth });
  return res.items;
}

/**
 * Retrieves all processing jobs across the system for monitoring/debugging.
 *
 * @returns Promise resolving to all processing jobs visible to the current
 *   caller. The result is an empty array when the service has no jobs.
 */
export async function listAllJobs() {
  const res = await jobClient.listAllJobs({});
  return res.jobs;
}
