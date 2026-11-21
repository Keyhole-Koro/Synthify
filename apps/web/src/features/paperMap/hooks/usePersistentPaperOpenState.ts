
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { DefaultOpenState, ExpansionMap } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import { hasPersistedSessionHint } from '@/features/auth/session';
import type { Workspace } from '@/features/workspaces/api';
import { clearOpenState, loadOpenState, saveOpenState } from '@/features/paperMap/expansionPersistence';
import { computeDefaultOpenState } from '@/features/paperMap/defaultOpenState';

/**
 * Configuration options for the {@link usePersistentPaperOpenState} hook.
 */
interface UsePersistentPaperOpenStateOptions {
  /** The currently authenticated user, or null if unauthenticated. */
  user: AuthUser | null;
  /** Whether the user session or workspace query is loading. */
  loading: boolean;
  /** List of workspaces loaded for the current user. */
  workspaces: Workspace[];
}

/** One resolved open state, and the canvas instance (key) that starts from it. */
interface Resolution {
  /** Also the canvas key: every resolution remounts the canvas with its state. */
  version: number;
  /** Whose state it is; the canvas built from it saves there. */
  owner: AuthUser | null;
  defaultOpenState: DefaultOpenState;
}

const ANONYMOUS_SCOPE = 'anonymous';

/**
 * Open-state persistence model: initial-load + save-only mirror.
 * - Resolving reads `defaultOpenState` from localStorage (or computes defaults) and bumps the
 *   canvas key in the same update, so PaperCanvas remounts with the state of the user it is
 *   shown to — never with the previous user's.
 * - PaperCanvas owns the live state. We mirror it via setState for app-side consumers
 *   (e.g. workspace-tree subtree fetcher), but never feed it back to PaperCanvas as a prop —
 *   that round-trip was the source of the open-state echo loop.
 * - Only the canvas of the latest resolution saves. Until it mounts, the outgoing canvas may
 *   still report changes (another user's papers, or state a logout just cleared); those are
 *   dropped.
 * - A user switch resolves again; an explicit reset resolves the logged-out defaults.
 *
 * @param options - User state, loading status, and workspaces list.
 * @returns State and updater handlers for managing canvas open state and focus persistence.
 */
export function usePersistentPaperOpenState({
  user,
  loading,
  workspaces,
}: UsePersistentPaperOpenStateOptions) {
  const [resolution, setResolution] = useState<Resolution | null>(null);
  const [resetCount, setResetCount] = useState(0);
  const [expansionMap, setExpansionMap] = useState<ExpansionMap>(() => new Map());
  const [focusedNodeId, setFocusedNodeId] = useState<string | null>(null);
  const latestFocusedNodeIdRef = useRef<string | null>(null);
  const workspacesRef = useRef(workspaces);
  const versionRef = useRef(-1);
  // The resolution whose canvas may save; -1 while a reset waits for its resolution.
  const activeVersionRef = useRef(-1);
  const resolvedForRef = useRef<{ scope: string; reset: number } | null>(null);
  const currentScope = user?.id ?? ANONYMOUS_SCOPE;
  // 直前ロードでログインしていたユーザーがリロードした場合は、Firebase の auth
  // 復元 (loading=false) を待ってから localStorage を読まないと、anonymous スコープの
  // 状態が一瞬描画されてからユーザー固有の状態に差し替わるチラつきが出る。
  // 逆にセッション hint が無いなら anonymous で確定なので即解決して LCP を稼ぐ。
  // useState の初期化関数で評価することで、SSR と CSR で表示が揺れないようにする。
  const [shouldWaitForAuth] = useState(() => hasPersistedSessionHint());
  const blockResolution = shouldWaitForAuth && loading;

  useLayoutEffect(() => {
    workspacesRef.current = workspaces;
  });

  useEffect(() => {
    if (blockResolution) return;
    const last = resolvedForRef.current;
    const resetting = last !== null && last.reset !== resetCount;
    if (last && !resetting && last.scope === currentScope) return;
    // A reset shows the logged-out defaults, even if the signed-out user is still on record.
    const owner = resetting ? null : user;
    resolvedForRef.current = { scope: owner?.id ?? ANONYMOUS_SCOPE, reset: resetCount };
    // Persisted state is authoritative: an absent entry means the user
    // explicitly closed everything there, not "show the default". Merging
    // with defaults would resurrect papers the user has closed, because
    // the sparse representation can't distinguish "never touched" from
    // "closed everything". Fall back to defaults only when there is no
    // persisted state at all. Workspace changes alone never re-resolve.
    const resolved = loadOpenState(owner) ?? computeDefaultOpenState({ user: owner, workspaces: workspacesRef.current });
    versionRef.current += 1;
    activeVersionRef.current = versionRef.current;
    latestFocusedNodeIdRef.current = resolved.focusedNodeId ?? null;
    setResolution({ version: versionRef.current, owner, defaultOpenState: resolved });
    setExpansionMap(resolved.expansionMap ?? new Map());
    setFocusedNodeId(resolved.focusedNodeId ?? null);
  }, [user, currentScope, blockResolution, resetCount]);

  const handleExpansionMapChange = useCallback((map: ExpansionMap) => {
    if (!resolution || resolution.version !== activeVersionRef.current) return;
    setExpansionMap(map);
    saveOpenState(resolution.owner, map, latestFocusedNodeIdRef.current);
  }, [resolution]);

  const handleFocusedNodeIdChange = useCallback((id: string | null) => {
    if (!resolution || resolution.version !== activeVersionRef.current) return;
    latestFocusedNodeIdRef.current = id;
    setFocusedNodeId(id);
  }, [resolution]);

  const resetToLoggedOutDefaults = useCallback((previousUser: AuthUser | null) => {
    clearOpenState(previousUser);
    // Nothing saves until the logged-out state is resolved and its canvas mounted.
    activeVersionRef.current = -1;
    setResetCount((count) => count + 1);
  }, []);

  return {
    canvasKey: resolution?.version ?? 0,
    defaultOpenState: resolution?.defaultOpenState,
    hasDefaultOpenState: resolution !== null,
    expansionMap,
    focusedNodeId,
    handleExpansionMapChange,
    handleFocusedNodeIdChange,
    resetToLoggedOutDefaults,
  };
}
