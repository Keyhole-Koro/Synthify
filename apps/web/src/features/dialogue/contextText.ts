import type { ContentNode, Paper, PaperId, PaperMap } from '@keyhole-koro/paper-in-paper';
import type { ChatContextPaper } from './api';

/** Keep a single paper's body well under the API's per-candidate cap. */
const MAX_CONTENT_CHARS = 4000;

/**
 * Collects the papers around `paperId` as context candidates.
 *
 * Structural closeness is the selector: the parent, the siblings and the
 * children. The canvas is the only side that knows what the reader currently
 * has around them, which is why this happens here rather than in the worker.
 */
export function collectCandidates(paperMap: PaperMap, paperId: PaperId): ChatContextPaper[] {
  const self = paperMap.get(paperId);
  if (!self) return [];

  const ids = new Set<PaperId>([paperId]);
  if (self.parentId) {
    ids.add(self.parentId);
    const parent = paperMap.get(self.parentId);
    parent?.childIds.forEach((id) => ids.add(id));
  }
  self.childIds.forEach((id) => ids.add(id));

  const out: ChatContextPaper[] = [];
  for (const id of ids) {
    const paper = paperMap.get(id);
    if (!paper) continue;
    out.push({
      paperId: id,
      title: paper.title,
      description: paper.description,
      content: extractText(paper).slice(0, MAX_CONTENT_CHARS),
    });
  }
  return out;
}

/**
 * Flattens paper content to plain text.
 *
 * Content is a string, a ContentNode tree, or live JSX. The first two flatten
 * cleanly; JSX does not, so those papers fall back to title and description.
 * That is a real loss of fidelity, but sending a React element is not an
 * option and the heading usually carries enough to reason about.
 */
export function extractText(paper: Paper): string {
  const content = paper.content;
  if (typeof content === 'string') return stripHTML(content);
  if (Array.isArray(content)) return nodesToText(content as ContentNode[]);
  return [paper.title, paper.description].filter(Boolean).join(' — ');
}

function stripHTML(html: string): string {
  if (typeof document === 'undefined') return html;
  const el = document.createElement('div');
  el.innerHTML = html;
  return (el.textContent ?? '').replace(/\s+/g, ' ').trim();
}

function nodesToText(nodes: ContentNode[]): string {
  return nodes
    .map(nodeToText)
    .filter(Boolean)
    .join('\n')
    .trim();
}

function nodeToText(node: ContentNode): string {
  switch (node.type) {
    case 'text':
      return node.value;
    case 'paragraph':
    case 'bold':
    case 'callout':
      return nodesToText(node.children);
    case 'section':
      return [node.title, nodesToText(node.children)].filter(Boolean).join('\n');
    case 'list':
      return node.items.map((item) => `- ${nodesToText(item)}`).join('\n');
    case 'table':
      return [node.headers.join(' | '), ...node.rows.map((r) => r.join(' | '))].join('\n');
    case 'paper-link':
      return node.label;
    case 'card':
      return `${node.title}: ${node.description}`;
    default:
      return '';
  }
}
