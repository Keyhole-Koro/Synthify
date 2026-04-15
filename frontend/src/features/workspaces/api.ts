import { callRPC } from '@/lib/rpc';

export interface Account {
  account_id: string;
  name: string;
  plan: string;
  storage_quota_bytes: number;
  storage_used_bytes: number;
  max_file_size_bytes: number;
  created_at: string;
}

export interface Workspace {
  workspace_id: string;
  account_id: string;
  name: string;
  created_at: string;
}

interface ConnectWorkspace {
  workspaceId: string;
  accountId: string;
  name: string;
  createdAt: string;
}

function mapWorkspace(workspace: ConnectWorkspace): Workspace {
  return {
    workspace_id: workspace.workspaceId,
    account_id: workspace.accountId,
    name: workspace.name,
    created_at: workspace.createdAt,
  };
}

export async function listWorkspaces(): Promise<Workspace[]> {
  const res = await callRPC<Record<string, never>, { workspaces: ConnectWorkspace[] }>(
    'WorkspaceService',
    'ListWorkspaces',
    {},
  );
  return (res.workspaces ?? []).map(mapWorkspace);
}

export async function getWorkspace(workspaceId: string): Promise<Workspace> {
  const res = await callRPC<{ workspaceId: string }, { workspace: ConnectWorkspace }>(
    'WorkspaceService',
    'GetWorkspace',
    { workspaceId },
  );
  return mapWorkspace(res.workspace);
}

export async function createWorkspace(name: string): Promise<Workspace> {
  const res = await callRPC<{ name: string }, { workspace: ConnectWorkspace }>(
    'WorkspaceService',
    'CreateWorkspace',
    { name },
  );
  return mapWorkspace(res.workspace);
}
