/**
 * Authentication gatekeeper component that protects the monitor UI behind Firebase Admin authentication.
 *
 * @module
 */

import React, { useCallback, useEffect, useState } from 'react';
import type { User } from 'firebase/auth';
import {
  authFetch,
  signInWithGoogle,
  signInWithPassword,
  signOutUser,
  subscribeUser,
} from '@/lib/auth-client';
import { PREVIEW_BYPASS, isAuthEmulator } from '@/client-config';

/**
 * State representing user authentication lifecycle in the gate.
 */
type GateState = 'loading' | 'signed-out' | 'checking' | 'authorized' | 'forbidden';

/**
 * Props for AuthGate component.
 */
interface AuthGateProps {
  /** Child content rendered once user has been authenticated and verified as admin. */
  children: React.ReactNode;
}

/**
 * Client-side authentication guard component that renders login options (Google / Password)
 * when unauthenticated, verifies admin claims against `/api/auth/me`, and renders children upon authorization.
 *
 * @param props - Child tree to render after Firebase authentication and admin verification succeed.
 * @returns Login/loading/error UI while authentication is pending, or the protected children.
 */
export function AuthGate({ children }: AuthGateProps) {
  const [state, setState] = useState<GateState>('loading');
  const [user, setUser] = useState<User | null>(null);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    return subscribeUser((u) => {
      setUser(u);
      if (!u) {
        setState('signed-out');
        return;
      }
      setState('checking');
      authFetch('/api/auth/me')
        .then((res) => setState(res.ok ? 'authorized' : 'forbidden'))
        .catch(() => setState('forbidden'));
    });
  }, []);

  const handleGoogle = useCallback(async () => {
    setError(null);
    setSubmitting(true);
    try {
      await signInWithGoogle();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sign-in failed');
    } finally {
      setSubmitting(false);
    }
  }, []);

  const handlePassword = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      setError(null);
      setSubmitting(true);
      try {
        await signInWithPassword(email, password);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Sign-in failed');
      } finally {
        setSubmitting(false);
      }
    },
    [email, password],
  );

  if (PREVIEW_BYPASS || state === 'authorized') {
    return <>{children}</>;
  }

  if (state === 'loading' || state === 'checking') {
    return (
      <div className="flex h-screen items-center justify-center bg-stone-50">
        <p className="text-sm text-stone-400">Loading…</p>
      </div>
    );
  }

  return (
    <div className="flex h-screen items-center justify-center bg-stone-50 px-4">
      <div className="w-full max-w-sm rounded-lg border border-stone-200 bg-white p-6 shadow-sm">
        <h1 className="text-sm font-bold uppercase tracking-tight text-stone-800">Synthify Monitor</h1>
        <p className="mt-0.5 text-[11px] text-stone-400">Admin access required</p>

        {state === 'forbidden' && (
          <div className="mt-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-[12px] text-red-700">
            {user?.email ? (
              <>
                <span className="font-medium">{user.email}</span> は管理者として許可されていません。
              </>
            ) : (
              'このアカウントには権限がありません。'
            )}
          </div>
        )}

        {error && (
          <div className="mt-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-[12px] text-red-700">
            {error}
          </div>
        )}

        <button
          onClick={handleGoogle}
          disabled={submitting}
          className="mt-5 w-full rounded-md bg-stone-800 px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-stone-700 disabled:opacity-50"
        >
          Google でサインイン
        </button>

        {isAuthEmulator() && (
          <form onSubmit={handlePassword} className="mt-4 space-y-2">
            <p className="text-[10px] font-semibold uppercase tracking-wider text-stone-400">
              Dev (Auth Emulator)
            </p>
            <input
              type="email"
              placeholder="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full rounded-md border border-stone-200 px-3 py-2 text-xs text-stone-800 focus:border-stone-400 focus:outline-none"
            />
            <input
              type="password"
              placeholder="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-md border border-stone-200 px-3 py-2 text-xs text-stone-800 focus:border-stone-400 focus:outline-none"
            />
            <button
              type="submit"
              disabled={submitting}
              className="w-full rounded-md border border-stone-300 px-4 py-2 text-xs font-medium text-stone-700 transition-colors hover:bg-stone-100 disabled:opacity-50"
            >
              メール/パスワードでサインイン
            </button>
          </form>
        )}

        {(state === 'forbidden' || user) && (
          <button
            onClick={() => signOutUser()}
            className="mt-4 w-full text-center text-[11px] text-stone-400 hover:text-stone-600"
          >
            サインアウト
          </button>
        )}
      </div>
    </div>
  );
}
