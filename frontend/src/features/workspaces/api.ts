import { callRPC } from '@/lib/rpc';

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

interface ConnectWorkspace {
  workspaceId: string;
  name: string;
  ownerId: string;
  plan: string;
  storageUsedBytes: number;
  storageQuotaBytes: number;
  maxFileSizeBytes?: number;
  maxUploadsPerDay?: number;
  createdAt: string;
}

interface ConnectWorkspaceMember {
  userId: string;
  email: string;
  role: string;
  isDev: boolean;
  invitedAt: string;
  invitedBy?: string;
}

function mapPlan(plan: string): Workspace['plan'] {
  return plan === 'WORKSPACE_PLAN_PRO' ? 'pro' : 'free';
}

function mapRole(role: string): WorkspaceMember['role'] {
  switch (role) {
    case 'WORKSPACE_ROLE_OWNER':
      return 'owner';
    case 'WORKSPACE_ROLE_EDITOR':
      return 'editor';
    default:
      return 'viewer';
  }
}

function mapWorkspace(workspace: ConnectWorkspace): Workspace {
  return {
    workspace_id: workspace.workspaceId,
    name: workspace.name,
    owner_id: workspace.ownerId,
    plan: mapPlan(workspace.plan),
    storage_used_bytes: workspace.storageUsedBytes,
    storage_quota_bytes: workspace.storageQuotaBytes,
    created_at: workspace.createdAt,
  };
}

function mapMember(member: ConnectWorkspaceMember): WorkspaceMember {
  return {
    user_id: member.userId,
    email: member.email,
    role: mapRole(member.role),
    is_dev: member.isDev,
    invited_at: member.invitedAt,
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

export async function getWorkspace(
  workspaceId: string,
): Promise<{ workspace: Workspace; members: WorkspaceMember[] }> {
  const res = await callRPC<
    { workspaceId: string },
    { workspace: ConnectWorkspace; members: ConnectWorkspaceMember[] }
  >('WorkspaceService', 'GetWorkspace', { workspaceId });
  return {
    workspace: mapWorkspace(res.workspace),
    members: (res.members ?? []).map(mapMember),
  };
}

export async function createWorkspace(name: string): Promise<Workspace> {
  const res = await callRPC<{ name: string }, { workspace: ConnectWorkspace }>(
    'WorkspaceService',
    'CreateWorkspace',
    { name },
  );
  return mapWorkspace(res.workspace);
}
