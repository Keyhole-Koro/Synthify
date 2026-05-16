'use client';

import { useState, useEffect } from 'react';
import { listWorkspaces, type Workspace } from '@/features/workspaces/api';
import { getInitialAuthUser, signInWithGoogleSession, subscribeAuthUser, type AuthUser } from '@/features/auth/session';

export function useAuthState() {
  const [user, setUser] = useState<AuthUser | null>(getInitialAuthUser);
  const [loading, setLoading] = useState(true);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [workspaceError, setWorkspaceError] = useState<Error | null>(null);

  useEffect(() => {
    return subscribeAuthUser((nextUser) => {
      setUser(nextUser);
      if (!nextUser) {
        setWorkspaces([]);
        setLoading(false);
        return;
      }

      setWorkspaceError(null);
      void listWorkspaces()
        .then(setWorkspaces)
        .catch((err) => {
          console.error('Failed to list workspaces:', err);
          setWorkspaceError(err instanceof Error ? err : new Error(String(err)));
        })
        .finally(() => setLoading(false));
    });
  }, []);

  async function handleGoogleSubmit() {
    setLoading(true);
    try {
      await signInWithGoogleSession();
    } catch (err) {
      console.error(err);
      alert('ログインに失敗しました。');
      setLoading(false);
    }
  }

  function handleEmailSubmit() {
    alert('メールアドレス認証は現在準備中です。Googleログインをご利用ください。');
  }

  return {
    user,
    loading,
    workspaces,
    workspaceError,
    setWorkspaces,
    handleGoogleSubmit,
    handleEmailSubmit,
  };
}
