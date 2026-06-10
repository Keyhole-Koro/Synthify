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

// 公開リンクビューア用: 認証ヘッダの代わりに share token を付けて tree を読む。
const publicTreeClient = createPublicRPCClient(TreeService);

export async function getTree(workspaceId: string) {
  const res = await treeClient.getTree({ workspaceId });
  return res.tree!;
}

// getTreeViaShareToken は無認証で、share token を header に載せて tree を取得する。
export async function getTreeViaShareToken(workspaceId: string, shareToken: string) {
  const res = await publicTreeClient.getTree(
    { workspaceId },
    { headers: { 'X-Synthify-Share-Token': shareToken } },
  );
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
