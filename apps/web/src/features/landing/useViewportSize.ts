'use client';

import { useEffect, useState } from 'react';

export interface ViewportSize {
  w: number;
  h: number;
}

export function useViewportSize() {
  const [hasMounted, setHasMounted] = useState(false);
  const [winSize, setWinSize] = useState<ViewportSize>({ w: 0, h: 0 });

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setHasMounted(true);
    const update = () => setWinSize({ w: window.innerWidth, h: window.innerHeight });
    update();
    window.addEventListener('resize', update);
    return () => window.removeEventListener('resize', update);
  }, []);

  return { hasMounted, winSize };
}
