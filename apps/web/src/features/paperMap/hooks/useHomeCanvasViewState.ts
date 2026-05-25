import { useMemo } from 'react';
import type { ExpansionMap, PaperId } from '@keyhole-koro/paper-in-paper';
import { AUTH_ID, ROOT_ID } from '@/features/paperMap/defaultOpenState';

// Root children that should NOT trigger the fullscreen canvas on their own.
// `auth` is the login/profile pane — surfacing the brand header & hero copy
// while it's the only thing open is intentional (it's part of the landing).
const FULLSCREEN_EXCLUDED_ROOT_CHILDREN: ReadonlySet<PaperId> = new Set([AUTH_ID]);

export function useHomeCanvasViewState(
  expansionMap: ExpansionMap,
  canvasFullscreen: boolean,
) {
  const rootOpenIds = expansionMap.get(ROOT_ID)?.openChildIds ?? [];
  const hasMeaningfulOpen = rootOpenIds.some(
    (id) => !FULLSCREEN_EXCLUDED_ROOT_CHILDREN.has(id),
  );

  return useMemo(() => ({
    isFullscreen: canvasFullscreen || hasMeaningfulOpen,
  }), [canvasFullscreen, hasMeaningfulOpen]);
}
