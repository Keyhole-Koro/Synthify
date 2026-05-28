'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { DefaultOpenState, ExpansionMap } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import { hasPersistedSessionHint } from '@/features/auth/session';
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
  const resolvedScopeRef = useRef<string | null>(null);
  const currentScope = user?.id ?? 'anonymous';
  // 直前ロードでログインしていたユーザーがリロードした場合は、Firebase の auth
  // 復元 (loading=false) を待ってから localStorage を読まないと、anonymous スコープの
  // 状態が一瞬描画されてからユーザー固有の状態に差し替わるチラつきが出る。
  // 逆にセッション hint が無いなら anonymous で確定なので即解決して LCP を稼ぐ。
  // useState の初期化関数で評価することで、SSR と CSR で表示が揺れないようにする。
  const [shouldWaitForAuth] = useState(() => hasPersistedSessionHint());
  const blockResolution = shouldWaitForAuth && loading;

  useEffect(() => {
    if (blockResolution) return;
    if (resolvedScopeRef.current === null) {
      resolvedScopeRef.current = currentScope;
      return;
    }
    if (resolvedScopeRef.current !== currentScope) {
      resolvedScopeRef.current = currentScope;
      setCanvasKey((prev) => prev + 1);
    }
  }, [currentScope, blockResolution]);

  useEffect(() => {
    if (blockResolution) return;
    // Persisted state is authoritative: an absent entry means the user
    // explicitly closed everything there, not "show the default". Merging
    // with defaults would resurrect papers the user has closed, because
    // the sparse representation can't distinguish "never touched" from
    // "closed everything". Fall back to defaults only when there is no
    // persisted state at all.
    const persisted = loadOpenState(user);
    const resolved = persisted ?? computeDefaultOpenState({ user, workspaces });
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDefaultOpenState(resolved);
    setExpansionMap(resolved.expansionMap ?? new Map());
    setFocusedNodeId(resolved.focusedNodeId ?? null);
    latestFocusedNodeIdRef.current = resolved.focusedNodeId ?? null;
  // Re-resolve only on canvasKey bump (user switch / explicit reset) or once auth
  // resolves (when we had to wait for it). workspaces mutations should not reset
  // canvas open-state.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canvasKey, blockResolution]);

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
    resolvedScopeRef.current = 'anonymous';
    setCanvasKey((prev) => prev + 1);
  }, []);

  return {
    canvasKey,
    defaultOpenState,
    hasDefaultOpenState: defaultOpenState !== undefined,
    expansionMap,
    focusedNodeId,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    resetToLoggedOutDefaults,
  };
}
