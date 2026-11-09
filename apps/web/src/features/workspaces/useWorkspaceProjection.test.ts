import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';
import type { Paper } from '@keyhole-koro/paper-in-paper';
import { ItemSchema, SubtreeItemSchema, type SubtreeItem } from '@/gen/proto/synthify/app/v1/tree_types_pb';
import { projectWorkspacePapers } from './useWorkspaceProjection';

function item(id: string, parentId: string, childIds: string[], fields: { title?: string; content?: string; description?: string } = {}): SubtreeItem {
  return create(SubtreeItemSchema, {
    item: create(ItemSchema, { id, parentId, childIds, title: fields.title ?? id, content: fields.content ?? '', description: fields.description ?? '' }),
    hasChildren: childIds.length > 0,
  });
}

const buildWsPaper = (workspaceId: string, childPapers: { id: string }[]): Paper => ({
  id: workspaceId,
  title: 'ws',
  description: '',
  content: '',
  parentId: 'workspaces',
  childIds: childPapers.map((p) => p.id),
});

function project(workspaceId: string, items: SubtreeItem[]) {
  const map = new Map(items.map((it) => [it.item!.id, it]));
  const roots = items.filter((it) => !it.item!.parentId).map((it) => it.item!.id);
  return new Map(projectWorkspacePapers(workspaceId, map, roots, buildWsPaper).map((p) => [p.id, p]));
}

describe('projectWorkspacePapers', () => {
  it('puts root items under the workspace and keeps only loaded children', () => {
    const papers = project('ws-a', [item('r', '', ['c1', 'missing'], { content: '<p>r</p>' }), item('c1', 'r', [], { content: '<p>c</p>' })]);
    expect(papers.get('r')).toMatchObject({ parentId: 'ws-a', childIds: ['c1'] });
    expect(papers.get('ws-a')!.childIds).toEqual(['r']);
  });

  it('escapes the description it falls back to when an item has no content', () => {
    const papers = project('ws-b', [item('r', '', [], { description: '<img src=x onerror=alert(1)>' })]);
    expect(papers.get('r')!.content).toBe('<p>&lt;img src=x onerror=alert(1)&gt;</p>');
  });

  it('reuses the previous paper object for an unchanged item, so only changed papers re-render', () => {
    const first = project('ws-c', [item('r', '', ['c1']), item('c1', 'r', [])]);
    // A refetch builds new message objects with the same values; one item is retitled.
    const second = project('ws-c', [item('r', '', ['c1']), item('c1', 'r', [], { title: 'renamed' })]);
    expect(second.get('r')).toBe(first.get('r'));
    expect(second.get('c1')).not.toBe(first.get('c1'));
    expect(second.get('c1')!.title).toBe('renamed');
  });
});
