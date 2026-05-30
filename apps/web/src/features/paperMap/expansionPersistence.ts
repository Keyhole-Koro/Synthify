import type { DefaultOpenState, ExpansionMap } from '@keyhole-koro/paper-in-paper';
import { log } from '@/lib/observability/log';

const STORAGE_KEY = 'synthify_expansion_map';
const FOCUS_KEY = 'synthify_focused_item_id';
const ANONYMOUS_SCOPE = 'anonymous';

export type OpenStateOwner = { id: string } | null;

function openStateScope(owner: OpenStateOwner) {
  return owner?.id ?? ANONYMOUS_SCOPE;
}

function storageKey(baseKey: string, scope?: string | null) {
  return `${baseKey}:${scope || ANONYMOUS_SCOPE}`;
}

function saveExpansionMap(map: ExpansionMap, scope: string) {
  if (typeof window === 'undefined') return;
  try {
    const entries = Array.from(map.entries()).map(([parentId, entry]) => [
      parentId,
      { openChildIds: entry.openChildIds },
    ]);
    const serialized = JSON.stringify(entries);
    localStorage.setItem(storageKey(STORAGE_KEY, scope), serialized);
  } catch (e) {
    log.error('Failed to save expansion map to localStorage', { source: 'expansion_persistence', mapSize: map.size }, e);
  }
}

function loadExpansionMap(scope: string): ExpansionMap | null {
  if (typeof window === 'undefined') return null;
  try {
    const serialized = localStorage.getItem(storageKey(STORAGE_KEY, scope));
    if (!serialized) return null;
    const entries = JSON.parse(serialized);
    return new Map(entries);
  } catch (e) {
    log.error('Failed to load expansion map from localStorage', { source: 'expansion_persistence' }, e);
    return null;
  }
}

function saveFocusedItemId(id: string | null, scope: string) {
  if (typeof window === 'undefined') return;
  const key = storageKey(FOCUS_KEY, scope);
  if (id === null) {
    localStorage.removeItem(key);
  } else {
    localStorage.setItem(key, id);
  }
}

function loadFocusedItemId(scope: string): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(storageKey(FOCUS_KEY, scope));
}

export function loadOpenState(owner: OpenStateOwner): DefaultOpenState | null {
  const scope = openStateScope(owner);
  const expansionMap = loadExpansionMap(scope);
  const focusedNodeId = loadFocusedItemId(scope);
  if (!expansionMap) return null;
  return { expansionMap, focusedNodeId };
}

export function saveOpenState(owner: OpenStateOwner, expansionMap: ExpansionMap, focusedNodeId: string | null) {
  const scope = openStateScope(owner);
  saveExpansionMap(expansionMap, scope);
  saveFocusedItemId(focusedNodeId, scope);
}

export function clearOpenState(owner: OpenStateOwner) {
  if (typeof window === 'undefined') return;
  const scope = openStateScope(owner);
  localStorage.removeItem(storageKey(STORAGE_KEY, scope));
  localStorage.removeItem(storageKey(FOCUS_KEY, scope));
}
