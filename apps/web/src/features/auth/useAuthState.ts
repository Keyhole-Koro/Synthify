'use client';

import { useCallback, useEffect, useState } from 'react';
import { listWorkspaces, type Workspace } from '@/features/workspaces/api';
import { signInUser } from '@/features/auth/userApi';
import { getInitialAuthUser, signInWithGoogleSession, subscribeAuthUser, type AuthUser } from '@/features/auth/session';
import { type AppError } from '@/lib/errors';
import { toAppError } from '@/lib/error_normalize';
import { log } from '@/lib/observability/log';

export function useAuthState() {
  const [user, setUser] = useState<AuthUser | null>(getInitialAuthUser);
  const [loading, setLoading] = useState(true);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [workspaceError, setWorkspaceError] = useState<AppError | null>(null);
  const [authError, setAuthError] = useState<AppError | null>(null);

  const fetchWorkspaces = useCallback(async () => {
    setWorkspaceError(null);
    try {
      // signInUser はサーバ側 users/accounts プロビジョニング (冪等)。
      // ListWorkspaces は workspaces テーブルだけを引くので users/accounts を待つ必要がなく、
      // 並列化することでリロード時のクリティカルパスを 1 往復分短縮できる。
      const [provision, ws] = await Promise.all([signInUser(), listWorkspaces()]);
      
      const accountId = provision.user?.accountId;
      if (accountId) {
        setUser((prev) => prev ? { ...prev, accountId } : null);
      }
      
      setWorkspaces(ws);
    } catch (err) {
      log.error('Failed to provision/list workspaces', { source: 'auth_fetch_workspaces' }, err);
      setWorkspaceError(toAppError(err));
    }
  }, []);

  useEffect(() => {
    return subscribeAuthUser(async (nextUser) => {
      setUser(nextUser);
      if (!nextUser) {
        setWorkspaces([]);
        setLoading(false);
        setAuthError(null);
        return;
      }

      setLoading(true);
      await fetchWorkspaces();
      setLoading(false);
    });
  }, [fetchWorkspaces]);

  const handleGoogleSubmit = useCallback(async () => {
    setLoading(true);
    setAuthError(null);
    try {
      await signInWithGoogleSession();
    } catch (err) {
      log.error('Google sign-in failed', { source: 'auth_google_signin' }, err);
      setAuthError(toAppError(err));
      setLoading(false);
    }
  }, []);

  return {
    user,
    loading,
    workspaces,
    workspaceError,
    authError,
    setWorkspaces,
    handleGoogleSubmit,
    retryWorkspaces: fetchWorkspaces,
  };
}
