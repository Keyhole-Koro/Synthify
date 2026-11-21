import { cleanup, render } from '@testing-library/react';
import { useCallback, useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ExpansionMap, Paper } from '@keyhole-koro/paper-in-paper';
import { ROOT_ID, WORKSPACES_ID } from '@/features/paperMap/defaultOpenState';
import type { Workspace } from '@/features/workspaces/api';
import { getTree } from '@/features/tree/api';
import { useWorkspaceTree } from './useWorkspaceTree';

// The tree never arrives: only whether it is asked for matters here.
vi.mock('@/features/tree/api', () => ({ getTree: vi.fn(() => new Promise(() => {})), getSubtree: vi.fn() }));
vi.mock('@/features/documents/api', () => ({ createDocument: vi.fn(), startProcessing: vi.fn(), uploadFile: vi.fn() }));
vi.mock('@/features/workspaces/paper/WorkspacePaper', () => ({ WorkspacePaper: () => null, WORKSPACE_PAPER_HUE: 200 }));
vi.mock('@/lib/observability/log', () => ({ log: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() } }));

const workspace = (workspaceId: string) => ({ workspaceId, name: workspaceId }) as unknown as Workspace;

// What a reload restores: the workspace list, the workspace and a paper inside it are open.
const RESTORED: ExpansionMap = new Map([
  [ROOT_ID, { openChildIds: [WORKSPACES_ID] }],
  [WORKSPACES_ID, { openChildIds: ['ws-1'] }],
  ['ws-1', { openChildIds: ['node-a'] }],
]);

// Stable stand-ins for the controller's callbacks.
const getWorkspaceName = (workspaceId: string) => workspaceId;
const noop = () => {};
const asyncNoop = async () => {};
const rename = async (workspaceId: string) => workspace(workspaceId);
const getRuntimeState = () => ({});

interface HarnessProps {
  workspaces: Workspace[];
  canFetch: (workspaceId: string) => boolean;
  onExpansionMapChange: (map: ExpansionMap) => void;
}

/** The controller's side of the hook: it mirrors the open state and keeps the workspace papers. */
function Harness({ workspaces, canFetch, onExpansionMapChange }: HarnessProps) {
  const [expansionMap, setExpansionMap] = useState(RESTORED);
  const [, setPaperGroups] = useState(() => new Map<string, Paper[]>());
  const handleExpansionMapChange = useCallback((map: ExpansionMap) => {
    onExpansionMapChange(map);
    setExpansionMap(map);
  }, [onExpansionMapChange]);
  const setWorkspacePapers = useCallback((workspaceId: string, papers: Paper[]) => {
    setPaperGroups((prev) => new Map(prev).set(workspaceId, papers));
  }, []);
  useWorkspaceTree(
    getWorkspaceName, expansionMap, handleExpansionMapChange, noop, setWorkspacePapers, noop,
    workspaces, rename, asyncNoop, asyncNoop, getRuntimeState, noop, canFetch,
  );
  return null;
}

/** An open-state listener that stops a render loop by throwing, instead of hanging the test. */
function boundedListener() {
  let calls = 0;
  return vi.fn((_map: ExpansionMap) => {
    calls += 1;
    if (calls > 10) throw new Error('the open state keeps changing');
  });
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('useWorkspaceTree', () => {
  it('leaves a restored workspace alone while ListWorkspaces has not confirmed access', () => {
    // Re-opening it used to report a new (equal) open state, which re-ran the expansion
    // watcher, which re-opened it: a render loop until access was confirmed.
    const onExpansionMapChange = boundedListener();
    render(<Harness workspaces={[workspace('ws-1')]} canFetch={() => false} onExpansionMapChange={onExpansionMapChange} />);
    expect(onExpansionMapChange).not.toHaveBeenCalled();
    expect(getTree).not.toHaveBeenCalled();
  });

  it('opens it, keeping what was open inside, once the fetched list confirms access', () => {
    const onExpansionMapChange = boundedListener();
    const { rerender } = render(
      <Harness workspaces={[workspace('ws-1')]} canFetch={() => false} onExpansionMapChange={onExpansionMapChange} />,
    );
    // ListWorkspaces lands: a new list, and the workspace is verified.
    rerender(<Harness workspaces={[workspace('ws-1')]} canFetch={() => true} onExpansionMapChange={onExpansionMapChange} />);
    expect(getTree).toHaveBeenCalledExactlyOnceWith('ws-1');
    expect(onExpansionMapChange).not.toHaveBeenCalled();
  });
});
