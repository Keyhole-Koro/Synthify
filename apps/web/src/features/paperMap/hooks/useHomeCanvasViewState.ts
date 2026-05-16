import { useMemo } from 'react';
import type { ExpansionMap } from '@keyhole-koro/paper-in-paper';
import { ROOT_ID } from '@/features/paperMap/staticPapers';

interface HomeCanvasViewState {
  isWorkspaceExpanded: boolean;
  isFullscreen: boolean;
}

export function useHomeCanvasViewState(
  expansionMap: ExpansionMap,
  canvasFullscreen: boolean,
): HomeCanvasViewState {
  const isWorkspaceExpanded = expansionMap.get(ROOT_ID)?.openChildIds.includes('workspaces') ?? false;

  return useMemo(() => ({
    isWorkspaceExpanded,
    isFullscreen: isWorkspaceExpanded || canvasFullscreen,
  }), [isWorkspaceExpanded, canvasFullscreen]);
}
