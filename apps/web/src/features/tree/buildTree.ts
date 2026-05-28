import type { Paper, PaperMap } from '@keyhole-koro/paper-in-paper';
import { ItemKind } from '@/gen/proto/synthify/app/v1/tree_types_pb';
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


/**
 * Returns the workspace_root item's id. Prefers the explicit kind flag
 * surfaced by the backend (PR 2 of the kind-track migration); falls back
 * to the legacy "level 0 and no parent" heuristic for items written
 * before PR 3 backfilled the column, or for backends that have not yet
 * been upgraded.
 */
export function findRootItemId(items: ApiItem[]): string | undefined {
  const explicit = items.find((i) => i.kind === ItemKind.WORKSPACE_ROOT);
  if (explicit) return explicit.id;
  const root = items.find((i) => i.level === 0 && !i.parentId);
  if (root) return root.id;
  return items.find((i) => !i.parentId)?.id;
}

/**
 * Returns ids of document_root items under the workspace_root. Useful for
 * the frontend to know which children of the workspace root represent
 * documents (as opposed to user-created nodes or future structural items).
 */
export function findDocumentRootItemIds(items: ApiItem[]): string[] {
  return items.filter((i) => i.kind === ItemKind.DOCUMENT_ROOT).map((i) => i.id);
}
