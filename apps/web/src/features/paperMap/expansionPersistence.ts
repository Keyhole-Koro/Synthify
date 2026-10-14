import type { DefaultOpenState, ExpansionMap } from '@keyhole-koro/paper-in-paper';
import { log } from '@/lib/observability/log';

const STORAGE_KEY = 'synthify_expansion_map';
const FOCUS_KEY = 'synthify_focused_item_id';
const ANONYMOUS_SCOPE = 'anonymous';

/**
 * Represents the owner of the canvas open state, typically an authenticated user or null.
 */
export type OpenStateOwner = { id: string } | null;

/**
 * Returns a storage scoping string derived from the owner's unique ID.
 */
function openStateScope(owner: OpenStateOwner) {
  return owner?.id ?? ANONYMOUS_SCOPE;
}

/**
 * Generates a scoped localStorage key.
 */
function storageKey(baseKey: string, scope?: string | null) {
  return `${baseKey}:${scope || ANONYMOUS_SCOPE}`;
}

/**
 * Serializes and writes the paper expansion map to browser localStorage.
 */
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

/**
 * Reads and deserializes the paper expansion map from browser localStorage.
 */
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

/**
 * Writes or removes the focused item ID in browser localStorage.
 */
function saveFocusedItemId(id: string | null, scope: string) {
  if (typeof window === 'undefined') return;
  const key = storageKey(FOCUS_KEY, scope);
  if (id === null) {
    localStorage.removeItem(key);
  } else {
    localStorage.setItem(key, id);
  }
}

/**
 * Retrieves the focused item ID from browser localStorage.
 */
function loadFocusedItemId(scope: string): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(storageKey(FOCUS_KEY, scope));
}

/**
 * Loads the persisted canvas open state (expansion map and focused node) for the given owner.
 *
 * @param owner - The current auth user or null for anonymous sessions.
 * @returns The restored open state, or null if no persisted record exists.
 */
export function loadOpenState(owner: OpenStateOwner): DefaultOpenState | null {
  const scope = openStateScope(owner);
  const expansionMap = loadExpansionMap(scope);
  const focusedNodeId = loadFocusedItemId(scope);
  if (!expansionMap) return null;
  return { expansionMap, focusedNodeId };
}

/**
 * Persists the canvas open state (expansion map and focused node) for the given owner.
 *
 * @param owner - The current auth user or null for anonymous sessions.
 * @param expansionMap - The current paper expansion map to serialize. Only open child IDs
 *   are persisted; runtime metadata is intentionally discarded.
 * @param focusedNodeId - The current focused node identifier, or null to clear focus.
 * @returns Nothing. Browser storage failures are logged by the lower-level persistence helper.
 */
export function saveOpenState(owner: OpenStateOwner, expansionMap: ExpansionMap, focusedNodeId: string | null) {
  const scope = openStateScope(owner);
  saveExpansionMap(expansionMap, scope);
  saveFocusedItemId(focusedNodeId, scope);
}

/**
 * Clears the persisted open state and focus records for the given owner from localStorage.
 *
 * @param owner - The auth user whose saved canvas state should be purged.
 * @returns Nothing. Safe to call during SSR or when no state has been saved.
 */
export function clearOpenState(owner: OpenStateOwner) {
  if (typeof window === 'undefined') return;
  const scope = openStateScope(owner);
  localStorage.removeItem(storageKey(STORAGE_KEY, scope));
  localStorage.removeItem(storageKey(FOCUS_KEY, scope));
}
