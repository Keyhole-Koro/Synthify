import { describe, expect, test } from 'bun:test';
import { expansionFor, hashFromIds, idsFromHash, openIds, readTree } from './tree';

const parents: Record<string, string | null> = { '/': null, '/api': '/', '/api/auth': '/api', '/ops': '/' };
const parentOf = (id: string) => parents[id];

describe('the #fragment', () => {
  test('round-trips ids, keeping slashes readable', () => {
    const hash = hashFromIds(['/api', '/api/auth', '/ops,1']);
    expect(hash).toBe('#open=/api,/api/auth,/ops%2C1');
    expect(idsFromHash(hash)).toEqual(['/api', '/api/auth', '/ops,1']);
  });

  test('is empty when nothing is open, and other fragments name nothing', () => {
    expect(hashFromIds([])).toBe('');
    expect(idsFromHash('#section-2')).toEqual([]);
    expect(idsFromHash('#open=%E0%A4%A,/api')).toEqual(['/api']);
  });
});

describe('open state', () => {
  test('a deep paper opens with every ancestor', () => {
    const map = expansionFor(['/api/auth'], parentOf);
    expect(map.get('/')?.openChildIds).toEqual(['/api']);
    expect(map.get('/api')?.openChildIds).toEqual(['/api/auth']);
  });

  test('reads back in the order it was written', () => {
    const ids = ['/ops', '/api', '/api/auth'];
    expect(openIds(expansionFor(ids, parentOf), '/')).toEqual(ids);
  });

  test('ignores ids that are not in the tree and rooms that are closed', () => {
    const map = expansionFor(['/nope'], parentOf);
    expect(map.size).toBe(0);
    map.set('/api', { openChildIds: ['/api/auth'] }); // /api itself is not open
    expect(openIds(map, '/')).toEqual([]);
  });
});

test('readTree needs the format and a root', () => {
  const paper = { id: '/', parentId: null, childIds: [], title: 'Root', description: '', content: '' };
  expect(readTree({ format: 'synthify-tree/1', title: '', lang: '', rootId: '/', papers: [paper] })).toMatchObject({ title: 'Root', lang: 'en' });
  expect(() => readTree({ format: 'other', papers: [] })).toThrow();
  expect(() => readTree({ format: 'synthify-tree/1', rootId: '/x', papers: [paper] })).toThrow();
});
