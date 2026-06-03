import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';
import { ItemSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import type { ApiItem } from '@/features/tree/api';
import { createWorkspaceTreeCache } from './workspaceTreeCache';

// makeItem builds an item under the given parent. Pass parentId '' for a root
// node (sitting directly under the workspace).
function makeItem(id: string, parentId: string, childIds: string[] = []): ApiItem {
  return create(ItemSchema, {
    id,
    parentId,
    title: id,
    childIds,
  });
}

describe('workspaceTreeCache', () => {
  it('replaces a workspace tree and tracks newly discovered root nodes', () => {
    const cache = createWorkspaceTreeCache();

    const first = cache.replaceWorkspaceTree('ws_1', [
      makeItem('node_1', '', ['child_1']),
      makeItem('child_1', 'node_1'),
    ]);

    expect(first.rootNodeIds).toEqual(['node_1']);
    expect(first.newRootNodeIds).toEqual(['node_1']);
    expect(cache.getRootNodeIds('ws_1')).toEqual(['node_1']);

    const second = cache.replaceWorkspaceTree('ws_1', [
      makeItem('node_1', '', ['child_1']),
      makeItem('child_1', 'node_1'),
      makeItem('node_2', ''),
    ]);

    expect(second.rootNodeIds).toEqual(['node_1', 'node_2']);
    expect(second.newRootNodeIds).toEqual(['node_2']);
  });

  it('injects a frontend-only mock root node', () => {
    const cache = createWorkspaceTreeCache();
    cache.replaceWorkspaceTree('ws_1', [
      makeItem('node_1', ''),
    ]);

    const itemId = cache.injectMockNode('ws_1', {
      itemId: 'debug_node_1',
      title: 'Debug Paper',
    });

    expect(itemId).toBe('debug_node_1');
    expect(cache.getRootNodeIds('ws_1')).toEqual(['node_1', 'debug_node_1']);
    expect(cache.getTreeItems('ws_1').get('debug_node_1')?.item?.title).toBe('Debug Paper');
    expect(cache.isLoaded('debug_node_1')).toBe(true);
  });
});
