import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';
import { ItemKind, ItemSchema, SubtreeItemSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import type { ApiItem } from '@/features/tree/api';
import { createWorkspaceTreeCache } from './workspaceTreeCache';

function makeItem(id: string, kind: ItemKind, childIds: string[] = []): ApiItem {
  return create(ItemSchema, {
    id,
    workspaceId: 'ws_1',
    title: id,
    kind,
    childIds,
  });
}

describe('workspaceTreeCache', () => {
  it('replaces a workspace tree and tracks newly discovered document roots', () => {
    const cache = createWorkspaceTreeCache();

    const first = cache.replaceWorkspaceTree('ws_1', [
      makeItem('root_1', ItemKind.WORKSPACE_ROOT, ['doc_root_1']),
      makeItem('doc_root_1', ItemKind.DOCUMENT_ROOT),
    ]);

    expect(first.rootItemId).toBe('root_1');
    expect(first.documentRootIds).toEqual(['doc_root_1']);
    expect(first.newDocumentRootIds).toEqual(['doc_root_1']);
    expect(cache.getRootItemId('ws_1')).toBe('root_1');
    expect(cache.getDocumentRootIds('ws_1')).toEqual(['doc_root_1']);

    const second = cache.replaceWorkspaceTree('ws_1', [
      makeItem('root_1', ItemKind.WORKSPACE_ROOT, ['doc_root_1', 'doc_root_2']),
      makeItem('doc_root_1', ItemKind.DOCUMENT_ROOT),
      makeItem('doc_root_2', ItemKind.DOCUMENT_ROOT),
    ]);

    expect(second.newDocumentRootIds).toEqual(['doc_root_2']);
  });

  it('merges a completed document root subtree into the cache', () => {
    const cache = createWorkspaceTreeCache();
    cache.replaceWorkspaceTree('ws_1', [
      makeItem('root_1', ItemKind.WORKSPACE_ROOT),
    ]);

    const documentRoot = create(SubtreeItemSchema, {
      item: makeItem('doc_root_1', ItemKind.DOCUMENT_ROOT),
      hasChildren: false,
    });
    const merged = cache.mergeDocumentRootItems('ws_1', 'doc_root_1', [documentRoot]);

    expect(merged?.workspaceRootItemId).toBe('root_1');
    expect(cache.getDocumentRootIds('ws_1')).toEqual(['doc_root_1']);
    expect(cache.getTreeItems('ws_1').get('doc_root_1')).toBe(documentRoot);
    expect(cache.isLoaded('doc_root_1')).toBe(true);
  });
});
