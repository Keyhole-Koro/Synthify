import { describe, expect, it } from 'vitest';
import { buildMockTree, resolveMockTreeSpec } from './mockTreeGenerator';

describe('buildMockTree', () => {
  it('produces a single root with every other node reachable from it', () => {
    const { items } = buildMockTree('ws', { totalItems: 200, depth: 4, branching: 3 });

    const roots = items.filter((item) => item.parentId === '');
    expect(roots).toHaveLength(1);

    const byId = new Map(items.map((item) => [item.id, item]));
    const seen = new Set<string>();
    const queue = [roots[0].id];
    while (queue.length > 0) {
      const id = queue.shift()!;
      if (seen.has(id)) continue;
      seen.add(id);
      queue.push(...byId.get(id)!.childIds);
    }
    expect(seen.size).toBe(items.length);
  });

  it('keeps parentId and childIds mutually consistent', () => {
    const { items } = buildMockTree('ws', { totalItems: 120, depth: 3, branching: 4 });
    const byId = new Map(items.map((item) => [item.id, item]));

    for (const item of items) {
      for (const childId of item.childIds) {
        expect(byId.get(childId)?.parentId).toBe(item.id);
      }
      if (item.parentId) {
        expect(byId.get(item.parentId)?.childIds).toContain(item.id);
        expect(item.level).toBe(byId.get(item.parentId)!.level + 1);
      }
    }
  });

  it('honours totalItems exactly, even when it truncates the balanced tree', () => {
    for (const totalItems of [1, 7, 250, 1001]) {
      const { items, resolved } = buildMockTree('ws', { totalItems, depth: 6, branching: 5 });
      expect(items).toHaveLength(totalItems);
      expect(resolved.totalItems).toBe(totalItems);
    }
  });

  it('fills the full balanced tree when totalItems is omitted', () => {
    const { items, resolved } = buildMockTree('ws', { depth: 3, branching: 4 });
    // 1 + 4 + 16 + 64
    expect(items).toHaveLength(85);
    expect(resolved.levelCounts).toEqual([1, 4, 16, 64]);
  });

  it('never exceeds the requested depth', () => {
    const { items } = buildMockTree('ws', { totalItems: 500, depth: 2, branching: 10 });
    expect(Math.max(...items.map((item) => item.level))).toBeLessThanOrEqual(2);
  });

  it('is deterministic for a given workspace id and spec', () => {
    const spec = { totalItems: 300, depth: 3, branching: 4, contentBytes: 1024, seed: 42 };
    const a = buildMockTree('ws', spec);
    const b = buildMockTree('ws', spec);
    expect(a.items.map((item) => [item.id, item.content])).toEqual(
      b.items.map((item) => [item.id, item.content]),
    );
    expect(a.resolved.contentTotalBytes).toBe(b.resolved.contentTotalBytes);
  });

  it('pads non-root content to roughly contentBytes', () => {
    const contentBytes = 4096;
    const { items } = buildMockTree('ws', { totalItems: 50, depth: 2, branching: 7, contentBytes });
    const children = items.filter((item) => item.parentId !== '');

    for (const child of children) {
      expect(child.content.length).toBeGreaterThanOrEqual(contentBytes);
      // The padding is emitted in whole paragraphs, so it overshoots by at most
      // one block rather than by an unbounded amount.
      expect(child.content.length).toBeLessThan(contentBytes * 1.5);
    }
  });

  it('leaves content short when contentBytes is 0', () => {
    const { items } = buildMockTree('ws', { totalItems: 20, depth: 2, branching: 4 });
    const child = items.find((item) => item.parentId !== '')!;
    expect(child.content.length).toBeLessThan(200);
  });

  it('reports the resolved spec so a measurement can name its input', () => {
    const resolved = resolveMockTreeSpec({ depth: 3, branching: 3 });
    expect(resolved).toMatchObject({ totalItems: 40, depth: 3, branching: 3, contentBytes: 0 });
  });
});
