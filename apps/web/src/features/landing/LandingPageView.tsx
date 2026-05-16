'use client';

import dynamic from 'next/dynamic';
import type { CSSProperties } from 'react';
import { ROOT_ID } from '@/features/paperMap/staticPapers';
import type { LandingPageController } from '@/features/landing/useLandingPageController';

const PaperCanvas = dynamic(
  () => import('@keyhole-koro/paper-in-paper').then((mod) => mod.PaperCanvas),
  { ssr: false },
);

function getCanvasFrameStyle(isFullscreen: boolean, winSize: { w: number; h: number }): CSSProperties {
  const chromeTransition = 'border-radius 0.2s ease, box-shadow 0.2s ease, outline-color 0.2s ease';
  if (isFullscreen || winSize.w === 0) {
    return { inset: 0, zIndex: 30, borderRadius: 0, boxShadow: 'none', transition: chromeTransition };
  }

  const w = Math.min(winSize.w * 0.95, winSize.h * 1.28);
  const h = Math.min(w / 2.05, winSize.h * 0.62);
  const cx = winSize.w / 2;
  const cy = winSize.h / 2;
  return {
    left: cx - w / 2,
    right: cx - w / 2,
    top: cy - h / 2,
    bottom: cy - h / 2,
    zIndex: 20,
    borderRadius: 16,
    boxShadow: '0 20px 40px -8px rgba(120,110,90,0.3)',
    outline: '1px solid rgba(180,170,155,0.6)',
    transition: chromeTransition,
  };
}

function LoadingScreen() {
  return (
    <div className="h-screen w-screen bg-[#f0e6d3] flex items-center justify-center">
      <div className="flex flex-col items-center gap-4">
        <div className="h-10 w-10 animate-spin rounded-full border-4 border-indigo-500/30 border-t-indigo-500" />
      </div>
    </div>
  );
}

function BrandHeader() {
  return (
    <div className="absolute left-6 top-6 z-20 flex select-none items-center gap-2.5">
      <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-500 shadow-md shadow-indigo-200">
        <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" />
        </svg>
      </div>
      <div className="flex flex-col">
        <span className="text-sm font-semibold tracking-tight text-stone-800">Synthify</span>
        <span className="text-[10px] uppercase tracking-[0.18em] text-stone-400">Document Tree Platform</span>
      </div>
    </div>
  );
}

function HeroCopy() {
  return (
    <div className="absolute left-1/2 top-[11%] z-10 -translate-x-1/2 text-center select-none whitespace-nowrap">
      <p className="text-[11px] font-semibold uppercase tracking-[0.22em] text-indigo-500/70 mb-2">Document Intelligence</p>
      <h1 className="text-3xl font-bold tracking-tight text-stone-800 sm:text-4xl">
        ドキュメントから、<span className="text-indigo-500">知識構造</span>へ
      </h1>
    </div>
  );
}

export function LandingPageView({ controller }: { controller: LandingPageController }) {
  if (!controller.isReady) return <LoadingScreen />;

  return (
    <div className="relative h-screen w-screen overflow-hidden" style={{ background: 'radial-gradient(ellipse at top left, #fff8ee 0%, #f0e6d3 50%, #e8dbc8 100%)' }}>
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -left-24 -top-24 h-[480px] w-[480px] rounded-full bg-amber-200/40 blur-[80px]" />
        <div className="absolute -bottom-32 right-1/4 h-[400px] w-[400px] rounded-full bg-orange-100/50 blur-[80px]" />
        <div className="absolute right-0 top-1/3 h-[300px] w-[300px] rounded-full bg-yellow-100/40 blur-[60px]" />
      </div>

      {!controller.isFullscreen && <BrandHeader />}
      {!controller.isFullscreen && <HeroCopy />}

      <div
        className="absolute overflow-hidden [contain:layout_paint] isolate"
        style={getCanvasFrameStyle(controller.isFullscreen, controller.winSize)}
      >
        <PaperCanvas
          key={controller.canvasKey}
          paperMap={controller.paperMap}
          rootId={ROOT_ID}
          expansionMap={controller.expansionMap}
          focusedNodeId={controller.focusedNodeId}
          isFullscreen={controller.isFullscreen}
          debug={false}
          onExpansionMapChange={controller.handleExpansionMapChange}
          onFocusedNodeIdChange={controller.handleFocusedNodeIdChange}
          onFullscreenChange={controller.setCanvasFullscreen}
        />
      </div>
    </div>
  );
}
