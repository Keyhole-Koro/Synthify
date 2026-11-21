import type { ApiItem } from './api';


/**
 * Returns the ids of root nodes: items that sit directly under the workspace
 * (no parent). The workspace itself is the tree root, so these top-level nodes
 * are what the workspace paper shows as its children.
 *
 * @param items - Flat item list returned by the tree API.
 * @returns Item IDs whose parent reference is empty; preserves the API input order.
 */
export function findRootNodeIds(items: ApiItem[]): string[] {
  return items.filter((i) => !i.parentId).map((i) => i.id);
}
