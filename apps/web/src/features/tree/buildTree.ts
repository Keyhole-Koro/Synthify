import type { Paper, PaperMap } from '@keyhole-koro/paper-in-paper';
import type { ApiItem } from './api';

const DEFAULT_HUE = 220;

export function buildPaperMapFromTree(items: ApiItem[]): PaperMap {
  return new Map(
    items.map((item) => [
      item.id,
      {
        ...item,
        content: item.content || `<p>${item.description}</p>`,
        hue: DEFAULT_HUE,
        parentId: item.parentId || null,
      } satisfies Paper,
    ]),
  );
}

/**
 * ツリー構造を持たない孤立アイテム（親なし・子なし）の ID 一覧を返す。
 * paper-in-paper の unplacedItemIds として使う。
 */
export function findUnplacedItemIds(items: ApiItem[]): string[] {
  const itemIds = new Set(items.map((i) => i.id));
  return items
    .filter((i) => i.level > 0 && (!i.parentId || !itemIds.has(i.parentId)))
    .map((i) => i.id);
}


/** Returns the root item ID (level 0 or a item without a parent). */
export function findRootItemId(items: ApiItem[]): string | undefined {
  const root = items.find((i) => i.level === 0 && !i.parentId);
  if (root) return root.id;
  // If no level-0 item exists, return a item without a parent.
  return items.find((i) => !i.parentId)?.id;
}
