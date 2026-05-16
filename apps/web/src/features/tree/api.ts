import { createRPCClient } from '@/lib/connect';
import { TreeService } from '@synthify/proto-ts/gen/synthify/tree/v1/tree_pb';
import { ItemService } from '@synthify/proto-ts/gen/synthify/tree/v1/item_pb';
import { JobService } from '@synthify/proto-ts/gen/synthify/tree/v1/job_pb';
import { TreeProjectionScope, type SubtreeItem } from '@synthify/proto-ts/gen/synthify/tree/v1/tree_types_pb';

export type { Item as ApiItem, SubtreeItem } from '@synthify/proto-ts/gen/synthify/tree/v1/tree_types_pb';
export type { EntityRef, TreeEntityDetail } from '@synthify/proto-ts/gen/synthify/tree/v1/item_pb';
export { TreeProjectionScope } from '@synthify/proto-ts/gen/synthify/tree/v1/tree_types_pb';

const treeClient = createRPCClient(TreeService);
const itemClient = createRPCClient(ItemService);
const jobClient = createRPCClient(JobService);

export async function getTree(
  workspaceId: string,
  opts: { levelFilters?: number[] } = {},
) {
  const res = await treeClient.getTree({
    workspaceId,
    levelFilters: opts.levelFilters ?? [],
  });
  return res.tree!;
}

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

export async function getSubtree(
  workspaceId: string,
  itemId: string,
  maxDepth = 3,
): Promise<SubtreeItem[]> {
  const res = await treeClient.getSubtree({ workspaceId, itemId, maxDepth });
  return res.items;
}

export async function listAllJobs() {
  const res = await jobClient.listAllJobs({});
  return res.jobs;
}
