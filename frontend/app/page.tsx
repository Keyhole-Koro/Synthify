'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import dynamic from 'next/dynamic';
import type { PaperViewState } from '@keyhole-koro/paper-in-paper';
import { type AuthMode } from '@/features/landing/AuthPaper';
import { buildLandingPaperMap, LANDING_ROOT_ID } from '@/features/landing/landingPaperMap';
import { signInWithPopup, GoogleAuthProvider, onAuthStateChanged, type User, signOut } from 'firebase/auth';
import { auth } from '@/lib/firebase';
import { listWorkspaces, type Workspace } from '@/features/workspaces/api';

type ExpansionMap = PaperViewState['expansionMap'];

const PaperCanvas = dynamic(
  () => import('@keyhole-koro/paper-in-paper').then((mod) => mod.PaperCanvas),
  { ssr: false },
);

const NOOP_PAPER_MAP_CHANGE = () => {};

export default function LandingPage() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [authMode, setAuthMode] = useState<AuthMode>('login');
  const [loading, setLoading] = useState(true); // Start true to check auth
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);

  useEffect(() => {
    return onAuthStateChanged(auth, async (u) => {
      setUser(u);
      if (u) {
        try {
          const ws = await listWorkspaces();
          setWorkspaces(ws);
        } catch (err) {
          console.error('Failed to list workspaces:', err);
        }
      }
      setLoading(false);
    });
  }, []);

  const handleEmailSubmit = useCallback(() => {
    alert('メールアドレス認証は現在準備中です。Googleログインをご利用ください。');
  }, []);

  const handleGoogleSubmit = useCallback(async () => {
    setLoading(true);
    try {
      const provider = new GoogleAuthProvider();
      await signInWithPopup(auth, provider);
      // user will be set by onAuthStateChanged
    } catch (err) {
      console.error(err);
      alert('ログインに失敗しました。');
      setLoading(false);
    }
  }, []);

  const handleLogout = useCallback(async () => {
    await signOut(auth);
  }, []);

  const handleEnterWorkspace = useCallback(() => {
    router.push('/workspaces');
  }, [router]);

  const paperMap = useMemo(
    () =>
      buildLandingPaperMap({
        user,
        workspaces,
        authMode,
        loading,
        onAuthModeChange: setAuthMode,
        onEmailSubmit: handleEmailSubmit,
        onGoogleSubmit: handleGoogleSubmit,
        onLogout: handleLogout,
        onEnterWorkspace: handleEnterWorkspace,
      }),
    [user, workspaces, authMode, loading, handleEmailSubmit, handleGoogleSubmit, handleLogout, handleEnterWorkspace],
  );

  const [expansionMap, setExpansionMap] = useState<ExpansionMap>(
    new Map([[LANDING_ROOT_ID, { openChildIds: ['auth'] }]]),
  );
  const [focusedNodeId, setFocusedNodeId] = useState<string | null>(null);

  return (
    <div className="relative h-screen w-screen overflow-hidden" style={{ background: 'radial-gradient(ellipse at top left, #fff8ee 0%, #f0e6d3 50%, #e8dbc8 100%)' }}>
      {/* subtle noise texture via radial highlights */}
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -left-24 -top-24 h-[480px] w-[480px] rounded-full bg-amber-200/40 blur-[80px]" />
        <div className="absolute -bottom-32 right-1/4 h-[400px] w-[400px] rounded-full bg-orange-100/50 blur-[80px]" />
        <div className="absolute right-0 top-1/3 h-[300px] w-[300px] rounded-full bg-yellow-100/40 blur-[60px]" />
      </div>

      {/* logo */}
      <div className="absolute left-6 top-6 z-20 flex select-none items-center gap-2.5">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-500 shadow-md shadow-indigo-200">
          <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" />
          </svg>
        </div>
        <div className="flex flex-col">
          <span className="text-sm font-semibold tracking-tight text-stone-800">Synthify</span>
          <span className="text-[10px] uppercase tracking-[0.18em] text-stone-400">Knowledge Graph Platform</span>
        </div>
      </div>

      {/* headline */}
      <div className="absolute left-1/2 top-[11%] z-10 -translate-x-1/2 text-center select-none whitespace-nowrap">
        <p className="text-[11px] font-semibold uppercase tracking-[0.22em] text-indigo-500/70 mb-2">Document Intelligence</p>
        <h1 className="text-3xl font-bold tracking-tight text-stone-800 sm:text-4xl">
          ドキュメントから、<span className="text-indigo-500">知識グラフ</span>へ
        </h1>
      </div>

      {/* canvas */}
      <div className="absolute left-1/2 top-1/2 aspect-[2.05/1] w-[min(95vw,128vh)] max-h-[62vh] -translate-x-1/2 -translate-y-[44%] overflow-hidden rounded-2xl shadow-xl shadow-stone-400/30 ring-1 ring-stone-300/60 [contain:layout_paint] isolate">
        <PaperCanvas
          paperMap={paperMap}
          rootId={LANDING_ROOT_ID}
          expansionMap={expansionMap}
          focusedNodeId={focusedNodeId}
          debug={false}
          onPaperMapChange={NOOP_PAPER_MAP_CHANGE}
          onExpansionMapChange={setExpansionMap}
          onFocusedNodeIdChange={setFocusedNodeId}
        />
      </div>

      {loading && (
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-stone-100/60 backdrop-blur-sm">
          <div className="flex flex-col items-center gap-4">
            <div className="h-10 w-10 animate-spin rounded-full border-4 border-indigo-500/30 border-t-indigo-500" />
            <p className="text-sm font-medium text-stone-500">接続中...</p>
          </div>
        </div>
      )}
    </div>
  );
}
