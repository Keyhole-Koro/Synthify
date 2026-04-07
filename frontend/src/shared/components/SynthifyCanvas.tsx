"use client";

import React, { useEffect, useState } from 'react';
import { PaperCanvas, type PaperMap } from '@keyhole-koro/paper-in-paper';
import { buildDemoPaperMap } from '@/demo/demoData';

export function SynthifyCanvas() {
  const [paperMap, setPaperMap] = useState<PaperMap | null>(null);

  useEffect(() => {
    // Client-side only
    setPaperMap(buildDemoPaperMap());
  }, []);

  if (!paperMap) {
    return <div className="flex items-center justify-center h-screen bg-slate-50 text-slate-400">Loading Canvas...</div>;
  }

  return (
    <div className="w-full h-screen overflow-hidden bg-white">
      <PaperCanvas 
        paperMap={paperMap}
        onPaperMapChange={setPaperMap}
        debug={process.env.NODE_ENV === 'development'}
      />
    </div>
  );
}
