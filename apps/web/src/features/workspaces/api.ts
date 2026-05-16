import { createRPCClient } from '@/lib/connect';
import { WorkspaceService } from '@synthify/proto-ts/gen/synthify/tree/v1/workspace_pb';

export type { Workspace } from '@synthify/proto-ts/gen/synthify/tree/v1/workspace_pb';

const client = createRPCClient(WorkspaceService);

export async function listWorkspaces() {
  const res = await client.listWorkspaces({});
  return res.workspaces;
}

export async function getWorkspace(workspaceId: string) {
  const res = await client.getWorkspace({ workspaceId });
  return res.workspace!;
}

export async function createWorkspace(name: string) {
  const res = await client.createWorkspace({ name });
  return res.workspace!;
}
