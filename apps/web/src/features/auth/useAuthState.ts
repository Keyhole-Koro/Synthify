'use client';

import { useCallback, useEffect, useState } from 'react';
import { listWorkspaces, type Workspace } from '@/features/workspaces/api';
import { signInUser } from '@/features/auth/userApi';
import { getInitialAuthUser, signInWithGoogleSession, subscribeAuthUser, type AuthUser } from '@/features/auth/session';
import { type AppError } from '@/lib/errors';
import { toAppError } from '@/lib/error_normalize';

export function useAuthState() {
  const [user, setUser] = useState<AuthUser | null>(getInitialAuthUser);
  const [loading, setLoading] = useState(true);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [workspaceError, setWorkspaceError] = useState<AppError | null>(null);
  const [authError, setAuthError] = useState<AppError | null>(null);

  const fetchWorkspaces = useCallback(async () => {
    setWorkspaceError(null);
    try {
      // サーバ側で users / accounts を provision してから workspace を取りに行く。
      await signInUser();
      const ws = await listWorkspaces();
      setWorkspaces(ws);
    } catch (err) {
      console.error('Failed to provision/list workspaces:', err);
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
      console.error(err);
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
