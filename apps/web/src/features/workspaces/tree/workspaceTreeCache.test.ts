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

  // GetTree returns an outline: bodies only on root nodes. These cases pin the
  // consequences, because getting them wrong is silent — the papers still
  // render, they just render the description fallback forever.
  it('treats only root nodes as body-loaded after an outline replace', () => {
    const cache = createWorkspaceTreeCache();
    cache.replaceWorkspaceTree('ws_1', [
      makeItem('root', '', ['child']),
      makeItem('child', 'root'),
    ]);

    expect(cache.isLoaded('root')).toBe(true);
    expect(cache.isLoaded('child')).toBe(false);
  });

  it('does not skip a subtree load just because the outline is present', () => {
    const cache = createWorkspaceTreeCache();
    cache.replaceWorkspaceTree('ws_1', [
      makeItem('root', '', ['child']),
      makeItem('child', 'root'),
    ]);

    expect(cache.isOutlineLoaded('ws_1')).toBe(true);
    expect(cache.shouldSkipSubtreeLoad('ws_1', 'child')).toBe(false);
  });

  it('skips a repeat load for an item whose body has arrived or is in flight', () => {
    const cache = createWorkspaceTreeCache();
    cache.replaceWorkspaceTree('ws_1', [makeItem('root', '', ['child']), makeItem('child', 'root')]);

    cache.markSubtreeLoading('child');
    expect(cache.shouldSkipSubtreeLoad('ws_1', 'child')).toBe(true);

    cache.markSubtreeLoadFinished('child');
    cache.markSubtreeLoaded('child');
    expect(cache.shouldSkipSubtreeLoad('ws_1', 'child')).toBe(true);
  });

  it('drops body-loaded marks for items a refresh replaced', () => {
    const cache = createWorkspaceTreeCache();
    cache.replaceWorkspaceTree('ws_1', [makeItem('root', '', ['child']), makeItem('child', 'root')]);
    cache.markSubtreeLoaded('child');

    // A treeChanged refresh rebuilds the workspace's items, so the body that
    // mark referred to is gone with them.
    cache.replaceWorkspaceTree('ws_1', [makeItem('root', '', ['child']), makeItem('child', 'root')]);

    expect(cache.isLoaded('child')).toBe(false);
    expect(cache.shouldSkipSubtreeLoad('ws_1', 'child')).toBe(false);
  });

  it('marks an injected mock tree fully body-loaded so it needs no backend', () => {
    const cache = createWorkspaceTreeCache();
    const { rootNodeIds } = cache.injectMockWorkspaceTree('debug_ws', { totalItems: 12, depth: 2, branching: 3 });

    const ids = Array.from(cache.getTreeItems('debug_ws').keys());
    expect(ids.length).toBe(12);
    expect(rootNodeIds).toHaveLength(1);
    for (const id of ids) {
      expect(cache.shouldSkipSubtreeLoad('debug_ws', id)).toBe(true);
    }
  });
});

