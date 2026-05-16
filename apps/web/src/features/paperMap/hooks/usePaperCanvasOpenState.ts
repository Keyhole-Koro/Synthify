import { useCallback, useEffect, useRef } from 'react';
import type { ExpansionMap } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import { saveOpenState } from '@/features/paperMap/expansionPersistence';

/**
 * Returns stable callbacks to pass to PaperCanvas that persist open-state to localStorage.
 */
export function usePaperCanvasOpenState(
  user: AuthUser | null,
  expansionMap: ExpansionMap,
  focusedNodeId: string | null,
) {
  const latestExpansionRef = useRef<ExpansionMap>(expansionMap);
  const latestFocusedNodeIdRef = useRef<string | null>(focusedNodeId);

  useEffect(() => {
    latestExpansionRef.current = expansionMap;
    latestFocusedNodeIdRef.current = focusedNodeId;
  }, [expansionMap, focusedNodeId]);

  const onExpansionMapChange = useCallback((expansionMap: ExpansionMap) => {
    latestExpansionRef.current = expansionMap;
    saveOpenState(user, expansionMap, latestFocusedNodeIdRef.current);
  }, [user]);

  const onFocusedNodeIdChange = useCallback((focusedNodeId: string | null) => {
    latestFocusedNodeIdRef.current = focusedNodeId;
    saveOpenState(user, latestExpansionRef.current, focusedNodeId);
  }, [user]);

  return { onExpansionMapChange, onFocusedNodeIdChange };
}
