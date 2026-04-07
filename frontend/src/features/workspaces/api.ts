import { callRPC } from '@/shared/lib/api';

export interface Workspace {
  workspace_id: string;
  name: string;
  owner_id: string;
  plan: 'free' | 'pro';
  storage_used_bytes: number;
  storage_quota_bytes: number;
  created_at: string;
}

export interface WorkspaceMember {
  user_id: string;
  email: string;
  role: 'owner' | 'editor' | 'viewer';
  is_dev: boolean;
  invited_at: string;
}

export async function listWorkspaces(): Promise<Workspace[]> {
  const res = await callRPC<Record<string, never>, { workspaces: Workspace[] }>(
    'WorkspaceService',
    'ListWorkspaces',
    {},
  );
  return res.workspaces ?? [];
}

export async function getWorkspace(
  workspaceId: string,
): Promise<{ workspace: Workspace; members: WorkspaceMember[] }> {
  return callRPC('WorkspaceService', 'GetWorkspace', { workspace_id: workspaceId });
}

export async function createWorkspace(name: string): Promise<Workspace> {
  const res = await callRPC<{ name: string }, { workspace: Workspace }>(
    'WorkspaceService',
    'CreateWorkspace',
    { name },
  );
  return res.workspace;
}
