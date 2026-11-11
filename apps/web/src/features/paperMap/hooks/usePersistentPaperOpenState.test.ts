import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { DefaultOpenState, ExpansionMap } from '@keyhole-koro/paper-in-paper';
import type { AuthUser } from '@/features/auth/session';
import { loadOpenState, saveOpenState } from '@/features/paperMap/expansionPersistence';
import { computeDefaultOpenState } from '@/features/paperMap/defaultOpenState';
import { usePersistentPaperOpenState } from './usePersistentPaperOpenState';

const session = vi.hoisted(() => ({ hint: false }));
vi.mock('@/features/auth/session', () => ({ hasPersistedSessionHint: () => session.hint }));
vi.mock('@/lib/observability/log', () => ({ log: { error: vi.fn() } }));

const alice: AuthUser = { id: 'alice', email: 'alice@example.com', name: 'Alice' };
const bob: AuthUser = { id: 'bob', email: 'bob@example.com', name: 'Bob' };

const expansion = (entries: Record<string, string[]>): ExpansionMap =>
  new Map(Object.entries(entries).map(([id, openChildIds]) => [id, { openChildIds }]));
const openIn = (state: DefaultOpenState | null | undefined, id: string) => state?.expansionMap?.get(id)?.openChildIds ?? [];

interface Props {
  user: AuthUser | null;
  loading: boolean;
}

function setup(initial: Partial<Props> & { user: AuthUser | null }) {
  const renders: Array<{ key: number; state: DefaultOpenState | undefined }> = [];
  const hook = renderHook((props: Props) => {
    const result = usePersistentPaperOpenState({ ...props, workspaces: [] });
    renders.push({ key: result.canvasKey, state: result.defaultOpenState });
    return result;
  }, { initialProps: { loading: false, ...initial } });
  return { ...hook, renders };
}

/** Every canvas key must come with exactly one state: the one the canvas mounts with. */
function statesByKey(renders: Array<{ key: number; state: DefaultOpenState | undefined }>) {
  const byKey = new Map<number, DefaultOpenState>();
  for (const { key, state } of renders) {
    if (!state) continue;
    const seen = byKey.get(key);
    if (seen) expect(state, `canvas key ${key} rendered with two different states`).toBe(seen);
    else byKey.set(key, state);
  }
  return byKey;
}

beforeEach(() => {
  localStorage.clear();
  session.hint = false;
});

describe('usePersistentPaperOpenState', () => {
  it('resolves the saved state of the user it starts with', () => {
    saveOpenState(alice, expansion({ workspaces: ['ws-alice'] }), 'ws-alice');
    const { result } = setup({ user: alice });
    expect(result.current.hasDefaultOpenState).toBe(true);
    expect(result.current.canvasKey).toBe(0);
    expect(openIn(result.current.defaultOpenState, 'workspaces')).toEqual(['ws-alice']);
    expect(result.current.focusedNodeId).toBe('ws-alice');
  });

  it("remounts the canvas with the new user's state in the same update as the key", () => {
    saveOpenState(alice, expansion({ workspaces: ['ws-alice'] }), null);
    saveOpenState(bob, expansion({ workspaces: ['ws-bob'] }), null);
    const { result, rerender, renders } = setup({ user: alice });
    rerender({ user: bob, loading: false });
    expect(result.current.canvasKey).toBe(1);
    const byKey = statesByKey(renders);
    expect(openIn(byKey.get(0), 'workspaces')).toEqual(['ws-alice']);
    expect(openIn(byKey.get(1), 'workspaces')).toEqual(['ws-bob']);
  });

  it('drops what the outgoing canvas reports after a user switch', () => {
    saveOpenState(alice, expansion({ workspaces: ['ws-alice'] }), null);
    saveOpenState(bob, expansion({ workspaces: ['ws-bob'] }), null);
    const { result, rerender } = setup({ user: alice });
    const outgoing = result.current;
    rerender({ user: bob, loading: false });

    act(() => {
      outgoing.handleFocusedNodeIdChange('ws-alice');
      outgoing.handleExpansionMapChange(expansion({ workspaces: ['ws-alice', 'secret'] }));
    });
    expect(openIn(loadOpenState(bob), 'workspaces')).toEqual(['ws-bob']);
    expect(openIn(loadOpenState(alice), 'workspaces')).toEqual(['ws-alice']);
    expect(result.current.expansionMap.get('workspaces')?.openChildIds).toEqual(['ws-bob']);
    expect(result.current.focusedNodeId).toBeNull();

    // The new canvas saves for its user.
    act(() => {
      result.current.handleFocusedNodeIdChange('ws-bob');
      result.current.handleExpansionMapChange(expansion({ workspaces: ['ws-bob', 'ws-bob-2'] }));
    });
    expect(loadOpenState(bob)).toEqual({ expansionMap: expansion({ workspaces: ['ws-bob', 'ws-bob-2'] }), focusedNodeId: 'ws-bob' });
  });

  it('a reset clears the signed-out user, shows the logged-out defaults and saves nothing for them', () => {
    saveOpenState(alice, expansion({ workspaces: ['ws-alice'] }), 'ws-alice');
    const { result, rerender } = setup({ user: alice });
    const outgoing = result.current;

    act(() => result.current.resetToLoggedOutDefaults(alice));
    expect(loadOpenState(alice)).toBeNull();
    expect(result.current.canvasKey).toBe(1);
    expect(result.current.defaultOpenState).toEqual(computeDefaultOpenState({ user: null, workspaces: [] }));
    act(() => outgoing.handleExpansionMapChange(expansion({ workspaces: ['ws-alice'] })));
    expect(loadOpenState(alice)).toBeNull();

    // The auth state catching up does not remount the canvas again.
    rerender({ user: null, loading: false });
    expect(result.current.canvasKey).toBe(1);
    act(() => result.current.handleExpansionMapChange(expansion({ root: ['about'] })));
    expect(openIn(loadOpenState(null), 'root')).toEqual(['about']);
    expect(loadOpenState(alice)).toBeNull();
  });

  it('waits for auth to restore when the last visit was signed in', () => {
    session.hint = true;
    saveOpenState(alice, expansion({ workspaces: ['ws-alice'] }), null);
    const { result, rerender, renders } = setup({ user: null, loading: true });
    expect(result.current.hasDefaultOpenState).toBe(false);
    rerender({ user: alice, loading: false });
    expect(result.current.canvasKey).toBe(0);
    expect(openIn(result.current.defaultOpenState, 'workspaces')).toEqual(['ws-alice']);
    // The anonymous state was never shown first.
    expect(statesByKey(renders).size).toBe(1);
  });

  it('falls back to the computed defaults when nothing was saved, and ignores workspace changes', () => {
    const { result, rerender } = setup({ user: alice });
    expect(result.current.defaultOpenState).toEqual(computeDefaultOpenState({ user: alice, workspaces: [] }));
    rerender({ user: alice, loading: false });
    expect(result.current.canvasKey).toBe(0);
  });
});
