import { describe, expect, it } from 'vitest';
import type { Paper, PaperMap } from '@keyhole-koro/paper-in-paper';
import { collectCandidates, extractText } from './contextText';
import { findOwningWorkspaceId } from './createDialogueChild';

function paper(over: Partial<Paper> & { id: string }): Paper {
  return {
    title: over.id,
    description: '',
    content: '',
    parentId: null,
    childIds: [],
    ...over,
  } as Paper;
}

function mapOf(...papers: Paper[]): PaperMap {
  return new Map(papers.map((p) => [p.id, p]));
}

describe('findOwningWorkspaceId', () => {
  it('walks up to the nearest workspace ancestor', () => {
    const map = mapOf(
      paper({ id: 'ws-1', childIds: ['node-a'] }),
      paper({ id: 'node-a', parentId: 'ws-1', childIds: ['node-b'] }),
      paper({ id: 'node-b', parentId: 'node-a' }),
    );

    expect(findOwningWorkspaceId(map, 'node-b', new Set(['ws-1']))).toBe('ws-1');
  });

  it('returns the paper itself when it is the workspace', () => {
    const map = mapOf(paper({ id: 'ws-1' }));

    expect(findOwningWorkspaceId(map, 'ws-1', new Set(['ws-1']))).toBe('ws-1');
  });

  // Product papers on the landing canvas sit outside any workspace; a dialogue
  // there has nothing to scope a turn to.
  it('returns null outside a workspace', () => {
    const map = mapOf(
      paper({ id: 'root', childIds: ['about'] }),
      paper({ id: 'about', parentId: 'root' }),
    );

    expect(findOwningWorkspaceId(map, 'about', new Set(['ws-1']))).toBeNull();
  });

  // A cycle would otherwise spin forever while walking parents.
  it('terminates on a parent cycle', () => {
    const map = mapOf(
      paper({ id: 'a', parentId: 'b' }),
      paper({ id: 'b', parentId: 'a' }),
    );

    expect(findOwningWorkspaceId(map, 'a', new Set(['ws-1']))).toBeNull();
  });
});

describe('collectCandidates', () => {
  it('gathers the paper, its parent, siblings and children', () => {
    const map = mapOf(
      paper({ id: 'parent', childIds: ['self', 'sibling'] }),
      paper({ id: 'self', parentId: 'parent', childIds: ['child'] }),
      paper({ id: 'sibling', parentId: 'parent' }),
      paper({ id: 'child', parentId: 'self' }),
      paper({ id: 'unrelated' }),
    );

    const ids = collectCandidates(map, 'self').map((c) => c.paperId).sort();

    expect(ids).toEqual(['child', 'parent', 'self', 'sibling']);
  });

  it('returns nothing for a paper that is not in the map', () => {
    expect(collectCandidates(mapOf(), 'ghost')).toEqual([]);
  });
});

describe('extractText', () => {
  it('flattens a ContentNode tree', () => {
    const p = paper({
      id: 'x',
      content: [
        { type: 'section', title: 'Background', children: [{ type: 'text', value: 'why it matters' }] },
        { type: 'list', items: [[{ type: 'text', value: 'first' }]] },
      ],
    });

    const got = extractText(p);

    expect(got).toContain('Background');
    expect(got).toContain('why it matters');
    expect(got).toContain('- first');
  });

  it('flattens table rows', () => {
    const p = paper({
      id: 'x',
      content: [{ type: 'table', headers: ['a', 'b'], rows: [['1', '2']] }],
    });

    expect(extractText(p)).toContain('a | b');
    expect(extractText(p)).toContain('1 | 2');
  });

  // JSX content cannot be flattened, so those papers contribute their heading
  // rather than nothing at all.
  it('falls back to title and description for JSX content', () => {
    const p = paper({
      id: 'x',
      title: 'Billing',
      description: 'plans and usage',
      content: { type: 'div' } as never,
    });

    const got = extractText(p);

    expect(got).toBe('Billing — plans and usage');
  });
});
