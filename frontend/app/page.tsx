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
    <div className="relative h-screen w-screen overflow-hidden" style={{ background: 'radial-gradient(circle at top left, rgba(255,248,233,0.95), transparent 28%), linear-gradient(180deg, #f6efe3 0%, #eee4d4 100%)' }}>
      <div className="absolute left-1/2 top-1/2 h-[clamp(360px,58vh,560px)] w-[min(1120px,calc(100vw-32px))] sm:w-[min(1120px,calc(100vw-64px))] -translate-x-1/2 -translate-y-1/2 overflow-hidden rounded-2xl [contain:layout_paint] isolate">
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
    </div>
  );
}
