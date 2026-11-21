import { describe, expect, it } from 'vitest';
import { AUTH_ID, ROOT_ID, WORKSPACES_ID, computeDefaultOpenState } from './defaultOpenState';

describe('computeDefaultOpenState', () => {
  it('opens the public documentation for anonymous users', () => {
    const state = computeDefaultOpenState({ user: null, workspaces: [] });
    expect(state.focusedNodeId).toBe(AUTH_ID);
    expect(state.expansionMap?.get(ROOT_ID)?.openChildIds).toEqual([AUTH_ID]);
  });

  it('opens workspaces and focuses them when an authenticated user has workspaces', () => {
    const state = computeDefaultOpenState({
      user: { id: 'user_1', email: 'u@example.com', name: 'User' },
      workspaces: [{ workspaceId: 'ws_1' } as never],
    });
    expect(state.focusedNodeId).toBe(WORKSPACES_ID);
    expect(state.expansionMap?.get(ROOT_ID)?.openChildIds).toEqual([AUTH_ID, WORKSPACES_ID]);
  });

  it('focuses auth when authenticated but no workspace exists', () => {
    const state = computeDefaultOpenState({
      user: { id: 'user_1', email: 'u@example.com', name: 'User' },
      workspaces: [],
    });
    expect(state.focusedNodeId).toBe(AUTH_ID);
  });
});
