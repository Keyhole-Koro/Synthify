import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  createWorkspace,
  updateWorkspace,
  deleteWorkspace,
  inviteMember,
  updateMemberRole,
  removeMember,
  type WorkspaceRole,
} from './api';
import { workspacesQueryKey, workspaceDetailQueryKey } from './queries';

/**
 * TanStack mutation hook for creating a new workspace, automatically invalidating
 * the workspace listing query on success.
 *
 * @returns TanStack Mutation instance for creating a workspace. The mutation argument is
 *   the display name; a successful mutation invalidates the workspace list cache.
 */
export function useCreateWorkspaceMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => createWorkspace(name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: workspacesQueryKey() });
    },
  });
}

/**
 * TanStack mutation hook for renaming or updating an existing workspace,
 * invalidating both the workspace list and the specific workspace detail query.
 *
 * @returns TanStack Mutation instance for updating a workspace. The mutation argument is
 *   `{ workspaceId, name }`; success invalidates both list and detail caches.
 */
export function useUpdateWorkspaceMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ workspaceId, name }: { workspaceId: string; name: string }) =>
      updateWorkspace(workspaceId, name),
    onSuccess: (updated) => {
      void queryClient.invalidateQueries({ queryKey: workspacesQueryKey() });
      void queryClient.invalidateQueries({ queryKey: workspaceDetailQueryKey(updated.workspaceId) });
    },
  });
}

/**
 * TanStack mutation hook for deleting a workspace, invalidating the workspace
 * list query and purging the detail query from cache.
 *
 * @returns TanStack Mutation instance for deleting a workspace. The mutation argument is
 *   the workspace ID; success invalidates the list and removes detail data.
 */
export function useDeleteWorkspaceMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (workspaceId: string) => deleteWorkspace(workspaceId),
    onSuccess: (_data, workspaceId) => {
      void queryClient.invalidateQueries({ queryKey: workspacesQueryKey() });
      queryClient.removeQueries({ queryKey: workspaceDetailQueryKey(workspaceId) });
    },
  });
}

/**
 * TanStack mutation hook for inviting a new member to a workspace by email.
 *
 * @param workspaceId - Target workspace identifier.
 * @returns TanStack Mutation instance for inviting a member. The mutation argument is
 *   `{ email, role }`; success refreshes the workspace member list.
 */
export function useInviteMemberMutation(workspaceId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ email, role }: { email: string; role: WorkspaceRole }) =>
      inviteMember(workspaceId, email, role),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: workspaceDetailQueryKey(workspaceId) });
    },
  });
}

/**
 * TanStack mutation hook for modifying a workspace member's permission role.
 *
 * @param workspaceId - Target workspace identifier.
 * @returns TanStack Mutation instance for updating a member's role. The mutation argument is
 *   `{ userId, role }`; success refreshes workspace membership data.
 */
export function useUpdateMemberRoleMutation(workspaceId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: WorkspaceRole }) =>
      updateMemberRole(workspaceId, userId, role),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: workspaceDetailQueryKey(workspaceId) });
    },
  });
}

/**
 * TanStack mutation hook for removing a member from a workspace.
 *
 * @param workspaceId - Target workspace identifier.
 * @returns TanStack Mutation instance for removing a member. The mutation argument is the
 *   member user ID; success refreshes the workspace member list.
 */
export function useRemoveMemberMutation(workspaceId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (userId: string) => removeMember(workspaceId, userId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: workspaceDetailQueryKey(workspaceId) });
    },
  });
}
