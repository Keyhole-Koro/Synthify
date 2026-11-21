import type { ExpansionMap, Paper } from '@keyhole-koro/paper-in-paper';

/** tree.json, as skills/synthify-tree/scripts/build.mjs writes it. */
export interface TreeFile {
  format: 'synthify-tree/1';
  title: string;
  lang: string;
  rootId: string;
  papers: TreePaper[];
}

export interface TreePaper {
  id: string;
  parentId: string | null;
  childIds: string[]; // in reading order
  title: string;
  description: string;
  content: string; // HTML
  css?: string;
}

export function readTree(value: unknown): TreeFile {
  const tree = value as Partial<TreeFile> | null;
  if (!tree || tree.format !== 'synthify-tree/1' || !Array.isArray(tree.papers)) throw new Error('tree.json is not a synthify-tree/1 file');
  const root = tree.papers.find((p) => p.id === tree.rootId && p.parentId === null);
  if (!root) throw new Error('tree.json names no root paper');
  return { format: tree.format, title: tree.title || root.title, lang: tree.lang || 'en', rootId: root.id, papers: tree.papers };
}

export function toPapers(tree: TreeFile): Paper[] {
  return tree.papers.map((p) => ({
    id: p.id,
    parentId: p.parentId,
    childIds: p.childIds,
    title: p.title,
    description: p.description,
    content: withImageMarkers(p.content),
    ...(p.css ? { overrideCss: p.css } : {}),
  }));
}

/**
 * Turns each <img> with a relative src (a file published with the tree) into a paper-in-paper
 * image marker, which the viewer fills with a data: URL. A paper's frame loads only https:,
 * data: and blob: images, and the page may be served over plain http (a local server).
 */
export function withImageMarkers(html: string): string {
  return html.replace(/(<img\b[^>]*?\s)src(\s*=\s*(["'])(?![a-z][a-z0-9+.-]*:|\/)[^"']*\3)/gi, '$1data-file-id$2');
}

// The open papers live in the page's #fragment ("#open=/api,/api/auth"), so a link or a reload
// opens the same ones, and SelfHostArtifacts shows the fragment in its address bar.
const OPEN = '#open=';

export function idsFromHash(hash: string): string[] {
  if (!hash.startsWith(OPEN)) return [];
  const ids: string[] = [];
  for (const part of hash.slice(OPEN.length).split(',')) {
    if (!part) continue;
    try {
      ids.push(decodeURIComponent(part));
    } catch {
      // A broken escape names no paper.
    }
  }
  return ids;
}

/** The fragment for these open papers, "" when none is. Slashes stay readable. */
export function hashFromIds(ids: string[]): string {
  return ids.length ? OPEN + ids.map((id) => encodeURIComponent(id).replace(/%2F/gi, '/')).join(',') : '';
}

/** The open state that shows these papers: each open in its parent, and so every ancestor. */
export function expansionFor(ids: string[], parentOf: (id: string) => string | null | undefined): ExpansionMap {
  const map: ExpansionMap = new Map();
  for (const id of ids) {
    const seen = new Set<string>();
    let child = id;
    for (let parent = parentOf(child); parent && !seen.has(parent); child = parent, parent = parentOf(child)) {
      seen.add(parent);
      const entry = map.get(parent) ?? { openChildIds: [] };
      if (!entry.openChildIds.includes(child)) entry.openChildIds = [...entry.openChildIds, child];
      map.set(parent, entry);
    }
  }
  return map;
}

/** The papers open under the root, parents before their children, each room in its layout order. */
export function openIds(expansionMap: ExpansionMap, rootId: string): string[] {
  const ids: string[] = [];
  const seen = new Set<string>([rootId]);
  const walk = (parent: string) => {
    for (const child of expansionMap.get(parent)?.openChildIds ?? []) {
      if (seen.has(child)) continue;
      seen.add(child);
      ids.push(child);
      walk(child);
    }
  };
  walk(rootId);
  return ids;
}
