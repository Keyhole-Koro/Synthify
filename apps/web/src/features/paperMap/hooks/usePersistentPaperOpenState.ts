'use client';

import { useCallback, useEffect, useState } from 'react';
import type { ExpansionMap } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import type { Workspace } from '@/features/workspaces/api';
import { clearOpenState, loadOpenState } from '@/features/paperMap/expansionPersistence';
import { computeDefaultOpenState } from '@/features/paperMap/defaultOpenState';
import { usePaperCanvasOpenState } from '@/features/paperMap/hooks/usePaperCanvasOpenState';

interface UsePersistentPaperOpenStateOptions {
  user: AuthUser | null;
  loading: boolean;
  workspaces: Workspace[];
}

export function usePersistentPaperOpenState({
  user,
  loading,
  workspaces,
}: UsePersistentPaperOpenStateOptions) {
  const [expansionMap, setExpansionMap] = useState<ExpansionMap>(() => new Map());
  const [focusedNodeId, setFocusedNodeId] = useState<string | null>(null);
  const [canvasKey, setCanvasKey] = useState(0);
  const ownerId = user?.id ?? null;

  useEffect(() => {
    if (loading) return;
    const persisted = loadOpenState(user);
    if (persisted) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setExpansionMap(persisted.expansionMap ?? new Map());
      setFocusedNodeId(persisted.focusedNodeId ?? null);
      return;
    }

    const defaults = computeDefaultOpenState({ user, workspaces });
    setExpansionMap(defaults.expansionMap ?? new Map());
    setFocusedNodeId(defaults.focusedNodeId ?? null);
  // Re-resolve only when the authenticated owner changes; workspace mutations should not reset canvas state.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loading, ownerId]);

  const {
    onExpansionMapChange: persistExpansionMap,
    onFocusedNodeIdChange: persistFocusedNodeId,
  } = usePaperCanvasOpenState(user, expansionMap, focusedNodeId);

  const handleExpansionMapChange = useCallback((map: ExpansionMap) => {
    setExpansionMap(map);
    persistExpansionMap(map);
  }, [persistExpansionMap]);

  const handleFocusedNodeIdChange = useCallback((id: string | null) => {
    setFocusedNodeId(id);
    persistFocusedNodeId(id);
  }, [persistFocusedNodeId]);

  const resetToLoggedOutDefaults = useCallback((previousUser: AuthUser | null) => {
    clearOpenState(previousUser);
    const defaults = computeDefaultOpenState({ user: null, workspaces: [] });
    setExpansionMap(defaults.expansionMap ?? new Map());
    setFocusedNodeId(defaults.focusedNodeId ?? null);
    setCanvasKey((prev) => prev + 1);
  }, []);

  return {
    canvasKey,
    expansionMap,
    focusedNodeId,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    resetToLoggedOutDefaults,
  };
}
