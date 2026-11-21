import { act, cleanup, render } from '@testing-library/react';
import { createRef } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { buildPaperMap, PaperCanvas, type PaperCanvasHandle } from '@keyhole-koro/paper-in-paper';
import { WorkspaceRootContent } from './WorkspaceRootContent';

const REPORT = '<h1>Report</h1><p><a data-paper-id="item-1">first</a></p><script>window.x = 1</script>';

function renderWorkspace() {
  const handle = createRef<PaperCanvasHandle>();
  const paperMap = buildPaperMap([
    {
      id: 'ws',
      title: 'Workspace',
      description: '',
      content: (
        <WorkspaceRootContent
          nodeId="ws"
          content={REPORT}
          overrideCss="h1 { color: teal; }"
          childItems={[{ id: 'item-1', title: 'First' }, { id: 'item-2', title: '' }]}
        />
      ),
      parentId: null,
      childIds: ['item-1', 'item-2'],
    },
    { id: 'item-1', title: 'First', description: '', content: '<p>1</p>', parentId: 'ws', childIds: [] },
    { id: 'item-2', title: '', description: '', content: '<p>2</p>', parentId: 'ws', childIds: [] },
  ]);
  render(
    <div style={{ height: 600 }}>
      <PaperCanvas ref={handle} paperMap={paperMap} labels={{ relatedLinks: '関連ノード' }} />
    </div>,
  );
  return handle;
}

beforeEach(() => {
  // jsdom has no layout: give the canvas a size so papers render.
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    const size = this.hasAttribute('data-paper-canvas') ? { width: 1000, height: 700 } : { width: 0, height: 0 };
    return { x: 0, y: 0, top: 0, left: 0, right: size.width, bottom: size.height, ...size, toJSON() {} } as DOMRect;
  });
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const reportFrame = () => document.querySelector<HTMLIFrameElement>('[data-testid="workspace-root-content"]')!;

/**
 * Plays the frame's bootstrap (jsdom does not run it): introduces itself with its document's
 * secret and a MessagePort, answers the canvas's liveness pings, and returns its end of the port.
 */
async function connect(frame: HTMLIFrameElement) {
  const secret = /const SECRET = '([^']*)'/.exec(frame.getAttribute('srcdoc') ?? '')?.[1] ?? '';
  const channel = new MessageChannel();
  channel.port1.onmessage = (event: MessageEvent) => {
    if (event.data?.type === 'ping') channel.port1.postMessage({ type: 'pong', seq: event.data.seq });
  };
  const hello = new MessageEvent('message', { data: { type: 'pip:hello', secret }, source: frame.contentWindow });
  Object.defineProperty(hello, 'ports', { value: [channel.port2] });
  await act(async () => {
    window.dispatchEvent(hello);
  });
  return channel.port1;
}

describe('WorkspaceRootContent', () => {
  it('renders the report in the sandboxed frame with the report styles and related links', async () => {
    renderWorkspace();
    await act(async () => {});
    const frame = reportFrame();
    expect(frame.getAttribute('sandbox')).toBe('allow-scripts');
    expect(frame.title).toBe('ワークスペースのレポート');
    const doc = frame.getAttribute('srcdoc')!;
    expect(doc).toContain('<h1>Report</h1>');
    expect(doc).toContain('Content-Security-Policy');
    expect(doc).toContain('h1 { color: teal; }');
    expect(doc).toContain('background: transparent');
    // item-2 is not linked from the report: it is appended, with the untitled fallback.
    expect(doc).toContain('関連ノード');
    expect(doc).toContain('<a data-paper-id="item-2">無題のノード</a>');
    expect(doc.match(/data-paper-id="item-1"/g)).toHaveLength(1);
  });

  it('opens a child paper when a link in the report is clicked', async () => {
    const handle = renderWorkspace();
    await act(async () => {});
    const port = await connect(reportFrame());
    try {
      port.postMessage({ type: 'open', paperId: 'item-1' });
      await vi.waitFor(() => {
        expect(handle.current!.getState().expansionMap.get('ws')?.openChildIds).toEqual(['item-1']);
      });
    } finally {
      port.close();
    }
  });

  it('ignores messages that do not come over the frame\'s port', async () => {
    const handle = renderWorkspace();
    await act(async () => {});
    await act(async () => {
      window.dispatchEvent(new MessageEvent('message', { data: { type: 'open', paperId: 'item-1' }, source: reportFrame().contentWindow }));
    });
    expect(handle.current!.getState().expansionMap.get('ws')).toBeUndefined();
  });
});
