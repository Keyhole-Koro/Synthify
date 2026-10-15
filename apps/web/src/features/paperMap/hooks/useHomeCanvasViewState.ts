import { useMemo } from 'react';
import type { ExpansionMap, PaperId } from '@keyhole-koro/paper-in-paper';
import { AUTH_ID, ROOT_ID } from '@/features/paperMap/defaultOpenState';

// Root children that should NOT trigger the fullscreen canvas on their own.
// `auth` is the login/profile pane — surfacing the brand header & hero copy
// while it's the only thing open is intentional (it's part of the landing).
const FULLSCREEN_EXCLUDED_ROOT_CHILDREN: ReadonlySet<PaperId> = new Set([AUTH_ID]);

/**
 * Derives whether the canvas view should transition into fullscreen mode based on
 * explicit user toggle or whether papers other than the auth/login card are currently opened.
 *
 * @param expansionMap - The current map of opened paper nodes.
 * @param canvasFullscreen - Explicit toggle state controlling fullscreen mode.
 * @returns An object containing the computed `isFullscreen` boolean flag.
 */
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
