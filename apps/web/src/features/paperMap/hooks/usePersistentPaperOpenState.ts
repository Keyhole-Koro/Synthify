'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { DefaultOpenState, ExpansionMap } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import type { Workspace } from '@/features/workspaces/api';
import { clearOpenState, loadOpenState, saveOpenState } from '@/features/paperMap/expansionPersistence';
import { computeDefaultOpenState } from '@/features/paperMap/defaultOpenState';

interface UsePersistentPaperOpenStateOptions {
  user: AuthUser | null;
  loading: boolean;
  workspaces: Workspace[];
}

/**
 * Open-state persistence model: initial-load + save-only mirror.
 * - On mount / canvasKey bump: resolve `defaultOpenState` from localStorage (or computed defaults)
 *   and hand it to PaperCanvas as initial state via key-remount.
 * - PaperCanvas owns the live state. We mirror it via setState for app-side consumers
 *   (e.g. workspace-tree subtree fetcher), but never feed it back to PaperCanvas as a prop —
 *   that round-trip was the source of the open-state echo loop.
 * - User switch & explicit reset bump `canvasKey` to remount PaperCanvas with a fresh defaultOpenState.
 */
export function usePersistentPaperOpenState({
  user,
  loading,
  workspaces,
}: UsePersistentPaperOpenStateOptions) {
  const [canvasKey, setCanvasKey] = useState(0);
  const [defaultOpenState, setDefaultOpenState] = useState<DefaultOpenState | undefined>(undefined);
  const [expansionMap, setExpansionMap] = useState<ExpansionMap>(() => new Map());
  const [focusedNodeId, setFocusedNodeId] = useState<string | null>(null);
  const latestFocusedNodeIdRef = useRef<string | null>(null);

  useEffect(() => {
    if (loading) return;
    const persisted = loadOpenState(user);
    const resolved = persisted ?? computeDefaultOpenState({ user, workspaces });
    setDefaultOpenState(resolved);
    setExpansionMap(resolved.expansionMap ?? new Map());
    setFocusedNodeId(resolved.focusedNodeId ?? null);
    latestFocusedNodeIdRef.current = resolved.focusedNodeId ?? null;
  // Re-resolve only on canvasKey bump (user switch / explicit reset) or once loading clears.
  // workspaces mutations should not reset canvas open-state.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canvasKey, loading]);

  const handleExpansionMapChange = useCallback((map: ExpansionMap) => {
    setExpansionMap(map);
    saveOpenState(user, map, latestFocusedNodeIdRef.current);
  }, [user]);

  const handleFocusedNodeIdChange = useCallback((id: string | null) => {
    latestFocusedNodeIdRef.current = id;
    setFocusedNodeId(id);
  }, []);

  const resetToLoggedOutDefaults = useCallback((previousUser: AuthUser | null) => {
    clearOpenState(previousUser);
    setCanvasKey((prev) => prev + 1);
  }, []);

  return {
    canvasKey,
    defaultOpenState,
    expansionMap,
    focusedNodeId,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    resetToLoggedOutDefaults,
  };
}
