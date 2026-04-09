'use client';

import { useCallback, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import dynamic from 'next/dynamic';
import type { PaperViewState } from '@keyhole-koro/paper-in-paper';
import { type AuthMode } from '@/features/landing/AuthPaper';
import { buildLandingPaperMap, LANDING_ROOT_ID } from '@/features/landing/landingPaperMap';

type ExpansionMap = PaperViewState['expansionMap'];

const PaperCanvas = dynamic(
  () => import('@/lib/PaperCanvasClient'),
  { ssr: false },
);

const NOOP_PAPER_MAP_CHANGE = () => {};

export default function LandingPage() {
  const router = useRouter();
  const [authMode, setAuthMode] = useState<AuthMode>('login');
  const [loading, setLoading] = useState(false);

  const handleAuthSubmit = useCallback(() => {
    setLoading(true);
    setTimeout(() => router.push('/w/ws_demo'), 800);
  }, [router]);

  const paperMap = useMemo(
    () =>
      buildLandingPaperMap({
        authMode,
        loading,
        onAuthModeChange: setAuthMode,
        onAuthSubmit: handleAuthSubmit,
      }),
    [authMode, loading, handleAuthSubmit],
  );

  const [expansionMap, setExpansionMap] = useState<ExpansionMap>(
    new Map([[LANDING_ROOT_ID, { openChildIds: ['auth'] }]]),
  );
  const [focusedNodeId, setFocusedNodeId] = useState<string | null>(null);

  return (
    <div className="relative h-screen w-screen overflow-hidden bg-transparent">
      <div className="absolute inset-x-8 bottom-8 top-[30vh]">
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
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-white/45 backdrop-blur-sm transition-opacity">
          <div className="flex flex-col items-center gap-4">
            <div className="h-10 w-10 animate-spin rounded-full border-4 border-indigo-500/30 border-t-indigo-500" />
            <p className="text-sm font-medium text-white/80">接続中...</p>
          </div>
        </div>
      )}

      <div className="absolute left-6 top-6 z-20 flex select-none items-center gap-2.5">
        <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-indigo-500">
          <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25"
            />
          </svg>
        </div>
        <div className="flex flex-col">
          <span className="text-sm font-semibold tracking-tight text-white/90">Synthify</span>
          <span className="text-[10px] uppercase tracking-[0.18em] text-white/45">Knowledge Graph Platform</span>
        </div>
      </div>
    </div>
  );
}
