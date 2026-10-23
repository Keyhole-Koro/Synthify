import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ItemSchema, SubtreeItemSchema } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import { createWorkspaceTreeCache } from './workspaceTreeCache';
import { createWorkspaceTreeCommands } from './workspaceTreeCommands';

const getTree = vi.fn();
const getSubtree = vi.fn();

vi.mock('@/features/tree/api', () => ({
  getTree: (...args: unknown[]) => getTree(...args),
  getSubtree: (...args: unknown[]) => getSubtree(...args),
}));

// The logger pulls in config/env, which validates the browser env at import
// time and has nothing to do with what these tests cover.
vi.mock('@/lib/observability/log', () => ({
  log: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

// GetTree returns an outline — bodies only on root nodes — so the body of any
// other node has to arrive through GetSubtree when its paper opens. These tests
// cover that hand-off, which the functional e2e cannot reach without a backend.
const outline = [
  create(ItemSchema, { id: 'root', parentId: '', title: 'Root', content: '<p>cover</p>', childIds: ['branch', 'leaf'] }),
  create(ItemSchema, { id: 'branch', parentId: 'root', title: 'Branch', content: '', childIds: ['grandchild'] }),
  create(ItemSchema, { id: 'leaf', parentId: 'root', title: 'Leaf', content: '' }),
  create(ItemSchema, { id: 'grandchild', parentId: 'branch', title: 'Grandchild', content: '' }),
];

function subtreeItem(id: string, parentId: string, content: string, hasChildren = false) {
  return create(SubtreeItemSchema, {
    item: create(ItemSchema, { id, parentId, title: id, content }),
    hasChildren,
  });
}

describe('workspaceTreeCommands with an outline-only GetTree', () => {
  beforeEach(() => {
    getTree.mockReset();
    getSubtree.mockReset();
    getTree.mockResolvedValue({ items: outline });
  });

  it('fetches a node body when its paper opens, and only once', async () => {
    const cache = createWorkspaceTreeCache();
    const commands = createWorkspaceTreeCommands(cache);
    await commands.refreshWorkspaceTree('ws_1');

    expect(cache.getTreeItems('ws_1').get('leaf')?.item?.content).toBe('');

    getSubtree.mockResolvedValue([subtreeItem('leaf', 'root', '<p>leaf body</p>')]);
    await commands.loadSubtree('ws_1', 'leaf');

    expect(getSubtree).toHaveBeenCalledTimes(1);
    expect(cache.getTreeItems('ws_1').get('leaf')?.item?.content).toBe('<p>leaf body</p>');

    // Opening the same paper again must not refetch.
    await commands.loadSubtree('ws_1', 'leaf');
    expect(getSubtree).toHaveBeenCalledTimes(1);
  });

  it('does not refetch a root node, whose body came with the outline', async () => {
    const cache = createWorkspaceTreeCache();
    const commands = createWorkspaceTreeCommands(cache);
    await commands.refreshWorkspaceTree('ws_1');

    await commands.loadSubtree('ws_1', 'root');

    expect(getSubtree).not.toHaveBeenCalled();
    expect(cache.getTreeItems('ws_1').get('root')?.item?.content).toBe('<p>cover</p>');
  });

  it('refetches bodies after a tree refresh replaced the items', async () => {
    const cache = createWorkspaceTreeCache();
    const commands = createWorkspaceTreeCommands(cache);
    await commands.refreshWorkspaceTree('ws_1');

    getSubtree.mockResolvedValue([subtreeItem('branch', 'root', '<p>v1</p>', true)]);
    await commands.loadSubtree('ws_1', 'branch');
    expect(getSubtree).toHaveBeenCalledTimes(1);

    // A treeChanged refresh discards the cached items, bodies included, so the
    // next open has to fetch again rather than trust a stale "loaded" mark.
    await commands.refreshWorkspaceTree('ws_1');
    getSubtree.mockResolvedValue([subtreeItem('branch', 'root', '<p>v2</p>', true)]);
    await commands.loadSubtree('ws_1', 'branch');

    expect(getSubtree).toHaveBeenCalledTimes(2);
    expect(cache.getTreeItems('ws_1').get('branch')?.item?.content).toBe('<p>v2</p>');
  });

  it('keeps the workspace usable when a body fetch fails', async () => {
    const cache = createWorkspaceTreeCache();
    const commands = createWorkspaceTreeCommands(cache);
    await commands.refreshWorkspaceTree('ws_1');

    getSubtree.mockRejectedValue(new Error('network down'));
    await expect(commands.loadSubtree('ws_1', 'leaf')).resolves.toEqual([]);

    // The failure must not leave the item stuck as "in flight", or opening it
    // again would silently never retry.
    expect(cache.isLoading('leaf')).toBe(false);
    expect(cache.shouldSkipSubtreeLoad('ws_1', 'leaf')).toBe(false);
  });
});
