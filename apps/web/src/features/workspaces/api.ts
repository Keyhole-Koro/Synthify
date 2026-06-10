import { create } from '@bufbuild/protobuf';
import { createRPCClient, createPublicRPCClient } from '@/lib/connect';
import { env } from '@/config/env';
import { getAuthHeaders } from '@/features/auth/session';
import {
  WorkspaceService,
  WorkspaceSchema,
  WorkspacePlan,
  type WorkspaceRole,
  type Workspace as WorkspaceType,
} from '@/gen/proto/synthify/app/v1/workspace_pb';

export type {
  Workspace,
  WorkspaceMember,
  ShareLink,
} from '@/gen/proto/synthify/app/v1/workspace_pb';
export { WorkspaceRole } from '@/gen/proto/synthify/app/v1/workspace_pb';

const client = createRPCClient(WorkspaceService);

// 公開リンク解決は無認証経路。auth ヘッダを付けない client を使う。
const publicClient = createPublicRPCClient(WorkspaceService);

export async function listWorkspaces() {
  const res = await client.listWorkspaces({});
  return res.workspaces;
}

export async function getWorkspace(workspaceId: string) {
  const res = await client.getWorkspace({ workspaceId });
  return { workspace: res.workspace!, members: res.members };
}

export async function createWorkspace(name: string) {
  const res = await client.createWorkspace({ name });
  return res.workspace!;
}

export async function updateWorkspace(workspaceId: string, name: string) {
  const res = await client.updateWorkspace({ workspaceId, name });
  return res.workspace!;
}

export async function deleteWorkspace(workspaceId: string) {
  await client.deleteWorkspace({ workspaceId });
}

// --- Members (invite-based sharing) --------------------------------------

export async function inviteMember(workspaceId: string, email: string, role: WorkspaceRole) {
  const res = await client.inviteMember({ workspaceId, email, role });
  return res.member!;
}

export async function updateMemberRole(workspaceId: string, userId: string, role: WorkspaceRole) {
  const res = await client.updateMemberRole({ workspaceId, userId, role });
  return res.member!;
}

export async function removeMember(workspaceId: string, userId: string) {
  await client.removeMember({ workspaceId, userId });
}

// --- Share links (public link sharing) -----------------------------------

export async function createShareLink(workspaceId: string, role: WorkspaceRole, expiresAt = '') {
  const res = await client.createShareLink({ workspaceId, role, expiresAt });
  return res.link!;
}

export async function listShareLinks(workspaceId: string) {
  const res = await client.listShareLinks({ workspaceId });
  return res.links;
}

export async function revokeShareLink(workspaceId: string, token: string) {
  await client.revokeShareLink({ workspaceId, token });
}

// resolveShareLink は無認証で呼ぶ。/view/[token] が使う。
export async function resolveShareLink(token: string) {
  const res = await publicClient.resolveShareLink({ token });
  return { workspace: res.workspace!, role: res.role };
}

// --- Dev workspace seed (local/dev only) ---------------------------------

interface DevSeedWorkspaceResponse {
  workspace: {
    workspace_id: string;
    name: string;
    account_id: string;
    plan?: string;
    storage_used_bytes?: number;
    storage_quota_bytes?: number;
    max_file_size_bytes?: number;
    max_uploads_per_5h?: number;
    max_uploads_per_1week?: number;
    created_at: string;
  };
}

export function canSeedDevWorkspace() {
  return env.nodeEnv === 'development' && !!env.firebase.authEmulatorUrl;
}

export async function seedDevWorkspace(): Promise<WorkspaceType> {
  const authHeaders = await getAuthHeaders();
  const res = await fetch(`${env.apiBaseUrl}/dev/seed-workspace`, {
    method: 'POST',
    headers: {
      ...authHeaders,
      'Content-Type': 'application/json',
    },
    body: '{}',
  });
  if (!res.ok) {
    throw new Error(await res.text());
  }
  const body = await res.json() as DevSeedWorkspaceResponse;
  const workspace = body.workspace;
  return create(WorkspaceSchema, {
    workspaceId: workspace.workspace_id,
    name: workspace.name,
    ownerId: workspace.account_id,
    plan: workspace.plan === 'pro' ? WorkspacePlan.PRO : WorkspacePlan.FREE,
    storageUsedBytes: BigInt(workspace.storage_used_bytes ?? 0),
    storageQuotaBytes: BigInt(workspace.storage_quota_bytes ?? 0),
    maxFileSizeBytes: BigInt(workspace.max_file_size_bytes ?? 0),
    maxUploadsPerDay: BigInt(workspace.max_uploads_per_1week ?? workspace.max_uploads_per_5h ?? 0),
    createdAt: workspace.created_at,
  });
}
