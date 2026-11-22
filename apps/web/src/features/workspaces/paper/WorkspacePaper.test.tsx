import { ConnectError } from '@connectrpc/connect';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';
import type { Workspace } from '@/features/workspaces/api';
import { setIntakeClosed } from '@/features/intake/intakeStatus';
import { createAppError } from '@/lib/errors';
import { WorkspacePaper } from './WorkspacePaper';

const mocks = vi.hoisted(() => ({
  session: {} as Record<string, unknown>,
}));

vi.mock('../session/useWorkspaceSession', () => ({
  useWorkspaceSession: () => mocks.session,
}));
vi.mock('@/features/workspaces/api', () => ({
  WorkspaceRole: { UNSPECIFIED: 0, EDITOR: 1, VIEWER: 2, OWNER: 3 },
  // No membership row: WorkspacePaper treats the caller as the owner.
  getWorkspace: vi.fn().mockResolvedValue({ members: [] }),
}));
vi.mock('@/features/documents/api', () => ({
  DocumentLifecycleState: { UPLOADED: 1, PROCESSING: 3, COMPLETED: 4 },
  listDocuments: vi.fn().mockResolvedValue([]),
  deleteDocument: vi.fn(),
}));
vi.mock('@/features/auth/session', () => ({ getInitialAuthUser: () => ({ id: 'owner' }) }));
vi.mock('@/features/sharing/SharePanel', () => ({ SharePanel: () => null }));
vi.mock('./components/WorkspaceRootContent', () => ({ WorkspaceRootContent: () => null }));

function session(overrides: Record<string, unknown> = {}) {
  return {
    jobStatus: undefined,
    jobStatusError: null,
    workspaceJobs: [],
    activeJobId: null,
    isRunning: false,
    isFailed: false,
    uploading: false,
    uploadMessage: null,
    isJustCompleted: false,
    isPopulated: false,
    handleUpload: vi.fn(),
    editingName: false,
    draftName: 'Research',
    savingName: false,
    nameError: null,
    setDraftName: vi.fn(),
    commitName: vi.fn(),
    startRename: vi.fn(),
    cancelRename: vi.fn(),
    ...overrides,
  };
}

const failedJob = {
  jobId: 'job_1',
  status: 'failed',
  errorMessage: 'E2E fixture worker failure',
} as unknown as FirestoreJobStatus;

function renderPaper(
  childItems: { id: string; title: string }[] = [],
  onDeleteWorkspace: () => Promise<void> = vi.fn(),
) {
  return render(
    <WorkspacePaper
      workspace={{ workspaceId: 'ws_1', name: 'Research' } as Workspace}
      workspaceId="ws_1"
      workspaceName="Research"
      hasTree={childItems.length > 0}
      childItems={childItems}
      rootContent={null}
      rootOverrideCss={null}
      onUploadFile={vi.fn()}
      onRenameWorkspace={vi.fn()}
      onDeleteWorkspace={onDeleteWorkspace}
      onSuggestedWorkspaceName={vi.fn()}
    />,
  );
}

const CLOSED = '新しい文書の受け付けは終了しました。';

describe('WorkspacePaper upload entry points', () => {
  beforeEach(() => {
    mocks.session = session();
  });

  // Unmount first: a mounted paper would react to the reset.
  afterEach(() => {
    cleanup();
    setIntakeClosed(false);
  });

  it('offers the upload drop for an empty workspace', () => {
    renderPaper();
    expect(screen.getByText('ファイルをアップロード')).toBeTruthy();
    expect(screen.queryByText(CLOSED)).toBeNull();
  });

  it('replaces the upload drop with a note once intake is closed', () => {
    setIntakeClosed(true);
    renderPaper();
    expect(screen.getByText(CLOSED)).toBeTruthy();
    expect(screen.queryByText('ファイルをアップロード')).toBeNull();
    expect(screen.queryByText('クリックまたはドラッグ&ドロップ')).toBeNull();
  });

  it('offers no retry for a failed job once intake is closed, but still shows the failure', () => {
    mocks.session = session({ activeJobId: 'job_1', jobStatus: failedJob, isFailed: true });
    renderPaper();
    expect(screen.getByText('再試行')).toBeTruthy();

    cleanup();
    setIntakeClosed(true);
    renderPaper();
    expect(screen.queryByText('再試行')).toBeNull();
    expect(screen.getByText(CLOSED)).toBeTruthy();
    expect(screen.getByText('E2E fixture worker failure')).toBeTruthy();
  });

  it('replaces "add a file" in a workspace that has a tree', () => {
    mocks.session = session({ isPopulated: true });
    const { container } = renderPaper([{ id: 'item_1', title: 'Overview' }]);
    fireEvent.mouseEnter(container.firstElementChild!);
    expect(screen.getByText('ファイルを追加')).toBeTruthy();

    cleanup();
    setIntakeClosed(true);
    const closed = renderPaper([{ id: 'item_1', title: 'Overview' }]);
    fireEvent.mouseEnter(closed.container.firstElementChild!);
    expect(screen.queryByText('ファイルを追加')).toBeNull();
    expect(screen.getByText(CLOSED)).toBeTruthy();
  });

  it('keeps sharing and deletion for the owner while intake is closed', () => {
    setIntakeClosed(true);
    renderPaper();
    expect(screen.getByRole('button', { name: '共有' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '削除' })).toBeTruthy();
  });
});

describe('WorkspacePaper deletion', () => {
  beforeEach(() => {
    mocks.session = session();
  });

  afterEach(() => {
    cleanup();
  });

  it('shows why the server refused, without the wrapper connect-es puts around it', async () => {
    // What the client call rejects with: the AppError lib/connect.ts builds,
    // wrapped by connect-es in a ConnectError with code Unknown.
    const refused = ConnectError.from(createAppError({
      kind: 'auth', code: 'PermissionDenied', message: 'only the owner can delete the workspace',
    }));
    renderPaper([], vi.fn().mockRejectedValue(refused));

    fireEvent.click(screen.getByRole('button', { name: '削除' }));
    fireEvent.click(screen.getByRole('button', { name: '削除を確定' }));

    expect(await screen.findByText('only the owner can delete the workspace')).toBeTruthy();
    expect(screen.queryByText(/\[unknown\]/)).toBeNull();
  });
});
