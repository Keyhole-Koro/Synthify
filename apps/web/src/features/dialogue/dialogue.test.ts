import { describe, expect, it } from 'vitest';
import type { Paper, PaperMap } from '@keyhole-koro/paper-in-paper';
import { collectCandidates, extractText } from './contextText';
import { findOwningWorkspaceId } from './createDialogueChild';
import { AUTO_COMMAND_TYPES, PROPOSAL_COMMAND_TYPES, parseCommands } from './commands';

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

describe('parseCommands', () => {
  it('keeps commands whose type is permitted for the field', () => {
    const raw = JSON.stringify([
      { type: 'FOCUS_NODE', nodeId: 'a' },
      { type: 'OPEN_NODE', parentId: 'a', childId: 'b' },
    ]);

    expect(parseCommands(raw, AUTO_COMMAND_TYPES)).toHaveLength(2);
  });

  // The worker already classifies, but `commands` is dispatched without the
  // reader seeing it, so the field that decides "apply silently" is checked on
  // both sides. A bug on either alone cannot auto-apply a structural change.
  it('drops a structural command that reached the auto field', () => {
    const raw = JSON.stringify([
      { type: 'FOCUS_NODE', nodeId: 'a' },
      { type: 'PATCH_NODE', nodeId: 'a' },
    ]);

    const got = parseCommands(raw, AUTO_COMMAND_TYPES);

    expect(got).toHaveLength(1);
    expect(got[0].type).toBe('FOCUS_NODE');
  });

  it('drops DELETE_NODE from either field', () => {
    const raw = JSON.stringify([{ type: 'DELETE_NODE', nodeId: 'a' }]);

    expect(parseCommands(raw, AUTO_COMMAND_TYPES)).toHaveLength(0);
    expect(parseCommands(raw, PROPOSAL_COMMAND_TYPES)).toHaveLength(0);
  });

  it('drops internal sync commands', () => {
    const raw = JSON.stringify([{ type: '__SYNC_PAPER_MAP' }, { type: '__SYNC_OPEN_STATE' }]);

    expect(parseCommands(raw, AUTO_COMMAND_TYPES)).toHaveLength(0);
  });

  it('yields nothing for malformed or absent payloads', () => {
    expect(parseCommands(undefined, AUTO_COMMAND_TYPES)).toEqual([]);
    expect(parseCommands('not json', AUTO_COMMAND_TYPES)).toEqual([]);
    expect(parseCommands('{"not":"an array"}', AUTO_COMMAND_TYPES)).toEqual([]);
    expect(parseCommands('[null, 3, "x"]', AUTO_COMMAND_TYPES)).toEqual([]);
  });
});
