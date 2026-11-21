import { useCallback, useEffect, useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getInitialAuthUser, signInWithGoogleSession, subscribeAuthUser, type AuthUser } from '@/features/auth/session';
import { loadWorkspaceSnapshot, saveWorkspaceSnapshot } from '@/features/workspaces/cache/workspaceSnapshotCache';
import { workspacesQueryOptions } from '@/features/workspaces/queries';
import { provisionUserQueryOptions } from '@/features/auth/queries';
import { log } from '@/lib/observability/log';

/**
 * React hook managing authentication state, reactive user subscriptions,
 * workspace synchronizations, and Google sign-in dispatch.
 *
 * @returns Authentication and workspace state plus actions for retrying the
 *   workspace query, marking a newly created workspace as verified, and
 *   starting Google sign-in.
 */
export function useAuthState() {
  const queryClient = useQueryClient();
  const [authUser, setUser] = useState<AuthUser | null>(getInitialAuthUser);
  const [authLoading, setAuthLoading] = useState(true);
  const [createdWorkspaceIds, setCreatedWorkspaceIds] = useState<Set<string>>(() => new Set());
  const [authError, setAuthError] = useState<Error | null>(null);

  // signInUser を先に済ませ、返ってきた課金アカウント ID をユーザーに付ける
  const provision = useQuery(provisionUserQueryOptions(authUser?.id));
  const accountId = provision.data?.user?.accountId;
  const user = useMemo(
    () => (authUser && accountId ? { ...authUser, accountId } : authUser),
    [authUser, accountId],
  );

  // TanStack Query でワークスペース一覧を取得・同期。スナップショットは取得までの
  // 仮表示（placeholderData）にとどめ、取得そのものは止めない
  const {
    data: workspaces = [],
    isFetched,
    error: queryError,
    refetch,
  } = useQuery({
    ...workspacesQueryOptions(authUser?.id, provision.isSuccess || provision.isError),
    placeholderData: () => (authUser ? loadWorkspaceSnapshot(authUser) : []),
  });

  // キャッシュが更新されたらローカルスナップショットにも自動永続化
  useEffect(() => {
    if (authUser && isFetched && workspaces.length > 0) {
      saveWorkspaceSnapshot(authUser, workspaces);
    }
  }, [authUser, isFetched, workspaces]);

  // 検証済みワークスペースIDは、API取得完了済みまたは手動マークされたIDから導出
  const verifiedWorkspaceIds = useMemo(() => {
    const ids = new Set(createdWorkspaceIds);
    if (isFetched) {
      for (const ws of workspaces) {
        ids.add(ws.workspaceId);
      }
    }
    return ids;
  }, [createdWorkspaceIds, isFetched, workspaces]);

  useEffect(() => {
    return subscribeAuthUser((nextUser) => {
      setUser(nextUser);
      setAuthLoading(false);
      if (!nextUser) {
        setCreatedWorkspaceIds(new Set());
        setAuthError(null);
        queryClient.removeQueries({ queryKey: ['workspaces'] });
        queryClient.removeQueries({ queryKey: ['auth', 'provision'] });
      }
    });
  }, [queryClient]);

  const retryWorkspaces = useCallback(async () => {
    await refetch();
  }, [refetch]);

  const markWorkspaceVerified = useCallback((workspaceId: string) => {
    setCreatedWorkspaceIds((prev) => {
      if (prev.has(workspaceId)) return prev;
      const next = new Set(prev);
      next.add(workspaceId);
      return next;
    });
  }, []);

  const handleGoogleSubmit = useCallback(async () => {
    setAuthLoading(true);
    setAuthError(null);
    const result = await signInWithGoogleSession();
    if (!result.ok) {
      log.error('Google sign-in failed', { source: 'auth_google_signin' }, result.error);
      setAuthError(new Error(result.error.message));
    }
    setAuthLoading(false);
  }, []);

  const workspaceError = queryError ?? null;
  const loading = authLoading || (Boolean(authUser) && !isFetched && workspaces.length === 0);

  return {
    user,
    loading,
    workspaces,
    workspaceError,
    authError,
    verifiedWorkspaceIds,
    markWorkspaceVerified,
    handleGoogleSubmit,
    retryWorkspaces,
  };
}
