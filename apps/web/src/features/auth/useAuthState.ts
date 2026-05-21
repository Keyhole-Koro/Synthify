'use client';

import { useCallback, useEffect, useState } from 'react';
import { listWorkspaces, type Workspace } from '@/features/workspaces/api';
import { signInUser } from '@/features/auth/userApi';
import { getInitialAuthUser, signInWithGoogleSession, subscribeAuthUser, type AuthUser } from '@/features/auth/session';

export function useAuthState() {
  const [user, setUser] = useState<AuthUser | null>(getInitialAuthUser);
  const [loading, setLoading] = useState(true);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [workspaceError, setWorkspaceError] = useState<Error | null>(null);

  useEffect(() => {
    return subscribeAuthUser(async (nextUser) => {
      setUser(nextUser);
      if (!nextUser) {
        setWorkspaces([]);
        setLoading(false);
        return;
      }

      setWorkspaceError(null);
      try {
        // サーバ側で users / accounts を provision してから workspace を取りに行く。
        // この順序を守らないと初回サインインユーザーで CreateWorkspace が account
        // 不在で失敗する。
        await signInUser();
        const ws = await listWorkspaces();
        setWorkspaces(ws);
      } catch (err) {
        console.error('Failed to provision/list workspaces:', err);
        setWorkspaceError(err instanceof Error ? err : new Error(String(err)));
      } finally {
        setLoading(false);
      }
    });
  }, []);

  const handleGoogleSubmit = useCallback(async () => {
    setLoading(true);
    try {
      await signInWithGoogleSession();
    } catch (err) {
      console.error(err);
      setLoading(false);
    }
  }, []);

  return {
    user,
    loading,
    workspaces,
    workspaceError,
    setWorkspaces,
    handleGoogleSubmit,
  };
}
