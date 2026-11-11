import { describe, expect, it } from 'vitest';
import { findRootNodeIds } from './buildTree';

describe('findRootNodeIds', () => {
  it('returns only items directly under the workspace in input order', () => {
    const ids = findRootNodeIds([
      { id: 'child', parentId: 'root' } as never,
      { id: 'root-a', parentId: '' } as never,
      { id: 'root-b', parentId: undefined } as never,
    ]);
    expect(ids).toEqual(['root-a', 'root-b']);
  });

  it('returns an empty list for an empty tree', () => {
    expect(findRootNodeIds([])).toEqual([]);
  });
});
