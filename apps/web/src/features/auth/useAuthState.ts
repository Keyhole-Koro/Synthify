'use client';

import { useCallback, useEffect, useState } from 'react';
import { listWorkspaces, type Workspace } from '@/features/workspaces/api';
import { signInUser } from '@/features/auth/userApi';
import { getInitialAuthUser, signInWithGoogleSession, subscribeAuthUser, type AuthUser } from '@/features/auth/session';
import { loadWorkspaceSnapshot, saveWorkspaceSnapshot } from '@/features/workspaces/cache/workspaceSnapshotCache';
import { type AppError } from '@/lib/errors';
import { toAppError } from '@/lib/error_normalize';
import { log } from '@/lib/observability/log';

export function useAuthState() {
  const [user, setUser] = useState<AuthUser | null>(getInitialAuthUser);
  const [loading, setLoading] = useState(true);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [verifiedWorkspaceIds, setVerifiedWorkspaceIds] = useState<Set<string>>(() => new Set());
  const [workspaceError, setWorkspaceError] = useState<AppError | null>(null);
  const [authError, setAuthError] = useState<AppError | null>(null);

  const fetchWorkspaces = useCallback(async (
    owner: AuthUser | null = null,
    options: { clearError?: boolean } = {},
  ) => {
    if (options.clearError ?? true) {
      setWorkspaceError(null);
    }
    try {
      // signInUser provisions users/accounts/account_users. ListWorkspaces and
      // TreeService authz both depend on account_users, so keep this serialized
      // to avoid a reload/cold-start race where the workspace exists but the
      // current user is not attached to an account yet.
      const provision = await signInUser();
      const ws = await listWorkspaces();
      
      const accountId = provision.user?.accountId;
      if (accountId) {
        setUser((prev) => prev ? { ...prev, accountId } : null);
      }
      
      setWorkspaces(ws);
      setVerifiedWorkspaceIds(new Set(ws.map((workspace) => workspace.workspaceId)));
      saveWorkspaceSnapshot(owner, ws);
      setWorkspaceError(null);
    } catch (err) {
      log.error('Failed to provision/list workspaces', { source: 'auth_fetch_workspaces' }, err);
      setVerifiedWorkspaceIds(new Set());
      setWorkspaceError(toAppError(err));
    }
  }, []);

  useEffect(() => {
    return subscribeAuthUser(async (nextUser) => {
      setUser(nextUser);
      if (!nextUser) {
        setWorkspaces([]);
        setVerifiedWorkspaceIds(new Set());
        setLoading(false);
        setAuthError(null);
        return;
      }

      setLoading(true);
      setVerifiedWorkspaceIds(new Set());
      setWorkspaces(loadWorkspaceSnapshot(nextUser));
      await fetchWorkspaces(nextUser);
      setLoading(false);
    });
  }, [fetchWorkspaces]);

  const retryWorkspaces = useCallback(async () => {
    await fetchWorkspaces(user, { clearError: false });
  }, [fetchWorkspaces, user]);

  const markWorkspaceVerified = useCallback((workspaceId: string) => {
    setVerifiedWorkspaceIds((prev) => {
      if (prev.has(workspaceId)) return prev;
      const next = new Set(prev);
      next.add(workspaceId);
      return next;
    });
  }, []);

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
    verifiedWorkspaceIds,
    markWorkspaceVerified,
    handleGoogleSubmit,
    retryWorkspaces,
  };
}
