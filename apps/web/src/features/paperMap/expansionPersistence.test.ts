import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/lib/observability/log', () => ({
  log: { error: vi.fn() },
}));

import { clearOpenState, loadOpenState, saveOpenState } from './expansionPersistence';

describe('expansion persistence', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('round-trips expansion and focus per authenticated owner', () => {
    const map = new Map([['root', { openChildIds: ['workspaces'] }]]);
    saveOpenState({ id: 'user_1' }, map, 'workspaces');

    expect(loadOpenState({ id: 'user_1' })).toEqual({
      expansionMap: map,
      focusedNodeId: 'workspaces',
    });
    expect(loadOpenState({ id: 'user_2' })).toBeNull();
  });

  it('supports anonymous state and clearing it', () => {
    saveOpenState(null, new Map([['root', { openChildIds: [] }]]), null);
    expect(loadOpenState(null)?.focusedNodeId).toBeNull();
    clearOpenState(null);
    expect(loadOpenState(null)).toBeNull();
  });

  it('returns null for malformed stored state', () => {
    localStorage.setItem('synthify_expansion_map:user_1', '{bad json');
    expect(loadOpenState({ id: 'user_1' })).toBeNull();
  });
});
