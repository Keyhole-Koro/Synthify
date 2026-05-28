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
 * Returns the workspace_root item's id. Schema invariant: every workspace
 * has exactly one item with kind=WORKSPACE_ROOT, so the lookup is direct.
 */
export function findRootItemId(items: ApiItem[]): string | undefined {
  return items.find((i) => i.kind === ItemKind.WORKSPACE_ROOT)?.id;
}

/**
 * Returns ids of document_root items under the workspace_root. Each document
 * has exactly one such item (enforced by document_tree_links).
 */
export function findDocumentRootItemIds(items: ApiItem[]): string[] {
  return items.filter((i) => i.kind === ItemKind.DOCUMENT_ROOT).map((i) => i.id);
}
