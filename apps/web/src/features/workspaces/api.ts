import { create } from '@bufbuild/protobuf';
import { createRPCClient, createPublicRPCClient } from '@/lib/connect';
import { env } from '@/config/env';
import { getAuthHeaders } from '@/features/auth/session';
import { UserFacingError } from '@/lib/errors';
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

/**
 * Fetches all workspaces accessible to the authenticated user.
 *
 * @returns Promise resolving to the workspaces visible to the authenticated user. The
 *   underlying Connect client applies the current auth context and may reject with a typed
 *   RPC error for unauthenticated or unavailable requests.
 */
export async function listWorkspaces() {
  const res = await client.listWorkspaces({});
  return res.workspaces;
}

/**
 * Retrieves details and member list for a specific workspace.
 *
 * @param workspaceId - Target workspace identifier.
 * @returns Object containing the workspace metadata and member entries. The generated
 *   response is expected to contain a workspace for a successful RPC response.
 */
export async function getWorkspace(workspaceId: string) {
  const res = await client.getWorkspace({ workspaceId });
  return { workspace: res.workspace!, members: res.members };
}

/**
 * Creates a new workspace owned by the current user.
 *
 * @param name - Display name for the workspace.
 * @returns The newly created workspace entity returned by the API.
 */
export async function createWorkspace(name: string) {
  const res = await client.createWorkspace({ name });
  return res.workspace!;
}

/**
 * Updates the display name of an existing workspace.
 *
 * @param workspaceId - Target workspace identifier.
 * @param name - New display name.
 * @returns The updated workspace entity after server-side validation and persistence.
 */
export async function updateWorkspace(workspaceId: string, name: string) {
  const res = await client.updateWorkspace({ workspaceId, name });
  return res.workspace!;
}

/**
 * Deletes a workspace and all of its associated trees and documents.
 *
 * @param workspaceId - Target workspace identifier.
 * @returns Promise fulfilled after the workspace and its associated server-side resources
 *   have been deleted.
 */
export async function deleteWorkspace(workspaceId: string) {
  await client.deleteWorkspace({ workspaceId });
}

// --- Members (invite-based sharing) --------------------------------------

/**
 * Invites a user by email to join a workspace with a specified role.
 *
 * @param workspaceId - Target workspace identifier.
 * @param email - Email address of the user to invite.
 * @param role - Assigned workspace permission role.
 * @returns The newly created workspace member record.
 */
export async function inviteMember(workspaceId: string, email: string, role: WorkspaceRole) {
  const res = await client.inviteMember({ workspaceId, email, role });
  return res.member!;
}

/**
 * Updates an existing member's role within a workspace.
 *
 * @param workspaceId - Target workspace identifier.
 * @param userId - Target user identifier.
 * @param role - New permission role.
 * @returns The updated workspace member record.
 */
export async function updateMemberRole(workspaceId: string, userId: string, role: WorkspaceRole) {
  const res = await client.updateMemberRole({ workspaceId, userId, role });
  return res.member!;
}

/**
 * Removes a member from a workspace.
 *
 * @param workspaceId - Target workspace identifier.
 * @param userId - Member user identifier to remove.
 * @returns Promise fulfilled after the member has been removed from the workspace.
 */
export async function removeMember(workspaceId: string, userId: string) {
  await client.removeMember({ workspaceId, userId });
}

// --- Share links (public link sharing) -----------------------------------

/**
 * Generates a public share link token for a workspace.
 *
 * @param workspaceId - Target workspace identifier.
 * @param role - Permission role granted by the share link.
 * @param expiresAt - Optional ISO-8601 expiration timestamp string.
 * @returns The generated share link record.
 */
export async function createShareLink(workspaceId: string, role: WorkspaceRole, expiresAt = '') {
  const res = await client.createShareLink({ workspaceId, role, expiresAt });
  return res.link!;
}

/**
 * Lists all active public share links configured for a workspace.
 *
 * @param workspaceId - Target workspace identifier.
 * @returns Array of share links.
 */
export async function listShareLinks(workspaceId: string) {
  const res = await client.listShareLinks({ workspaceId });
  return res.links;
}

/**
 * Revokes an existing public share link token.
 *
 * @param workspaceId - Target workspace identifier.
 * @param token - Token string of the share link to revoke.
 * @returns Promise fulfilled after the share link has been revoked and can no longer grant access.
 */
export async function revokeShareLink(workspaceId: string, token: string) {
  await client.revokeShareLink({ workspaceId, token });
}

// resolveShareLink は無認証で呼ぶ。/view/[token] が使う。
/**
 * Resolves a public share link token unauthenticatedly, returning workspace metadata and granted role.
 *
 * @param token - Public share link token.
 * @returns Object containing the workspace entity and the granted role.
 */
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

/**
 * Determines whether the local development environment supports seeding demo workspaces.
 *
 * @returns True only when the app runs in development mode and the Firebase Auth emulator
 *   URL is configured; production builds therefore never expose the seed action.
 */
export function canSeedDevWorkspace() {
  return env.nodeEnv === 'development' && !!env.firebase.authEmulatorUrl;
}

/**
 * Requests the dev server to create and seed a pre-populated workspace with sample documents and trees.
 *
 * @returns The seeded workspace Protobuf entity after converting the JSON seed response to
 *   the generated schema type.
 * @throws UserFacingError when the dev endpoint returns a non-2xx response; network errors
 *   from `fetch` are propagated to the caller.
 */
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
    throw new UserFacingError('開発用ワークスペースの作成に失敗しました。');
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
