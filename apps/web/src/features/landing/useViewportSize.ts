
import { useEffect, useState } from 'react';

interface ViewportSize {
  w: number;
  h: number;
}

/**
 * Tracks the browser window viewport dimensions and client-side mount status.
 *
 * @returns Object with `hasMounted` and `{ w, h }` dimensions. Dimensions are `{0, 0}` during
 *   SSR and before the first client-side effect; the resize listener is removed on unmount.
 */
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
