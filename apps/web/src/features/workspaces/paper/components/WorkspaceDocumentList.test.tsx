import { ConnectError } from '@connectrpc/connect';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createAppError } from '@/lib/errors';
import { WorkspaceDocumentList } from './WorkspaceDocumentList';

// What a client call rejects with: the AppError that lib/connect.ts builds,
// wrapped by connect-es in a ConnectError with code Unknown.
function refused(kind: 'validation' | 'auth', code: string, message: string) {
  return ConnectError.from(createAppError({ kind, code, message }));
}

const api = vi.hoisted(() => ({
  listDocuments: vi.fn(),
  deleteDocument: vi.fn(),
}));

vi.mock('@/features/documents/api', () => ({
  DocumentLifecycleState: { UPLOADED: 1, PROCESSING: 3, COMPLETED: 4 },
  listDocuments: api.listDocuments,
  deleteDocument: api.deleteDocument,
}));

function doc(documentId: string, uploadedBy: string, status = 4) {
  return { documentId, workspaceId: 'ws_1', uploadedBy, filename: `${documentId}.pdf`, status };
}

function renderList(overrides: Partial<React.ComponentProps<typeof WorkspaceDocumentList>> = {}) {
  const onDeleted = vi.fn();
  render(
    <WorkspaceDocumentList
      workspaceId="ws_1"
      currentUserId="editor"
      canManage={false}
      canWrite
      refreshKey=""
      onDeleted={onDeleted}
      {...overrides}
    />,
  );
  return { onDeleted };
}

describe('WorkspaceDocumentList', () => {
  beforeEach(() => {
    api.listDocuments.mockReset();
    api.deleteDocument.mockReset();
  });

  it('renders nothing for a workspace without documents', async () => {
    api.listDocuments.mockResolvedValue([]);
    renderList();
    await waitFor(() => expect(api.listDocuments).toHaveBeenCalledWith('ws_1'));
    expect(screen.queryByTestId('workspace-document-list')).toBeNull();
  });

  it('lets an editor delete only their own uploads', async () => {
    api.listDocuments.mockResolvedValue([doc('mine', 'editor'), doc('theirs', 'owner')]);
    renderList();
    await screen.findByText('mine.pdf');
    expect(screen.getByRole('button', { name: 'mine.pdf を削除' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'theirs.pdf を削除' })).toBeNull();
  });

  it('lets an owner delete every document, but not while it is processing', async () => {
    api.listDocuments.mockResolvedValue([doc('a', 'editor'), doc('b', 'editor', 3)]);
    renderList({ canManage: true, currentUserId: 'owner' });
    await screen.findByText('a.pdf');
    expect((screen.getByRole('button', { name: 'a.pdf を削除' }) as HTMLButtonElement).disabled).toBe(false);
    expect((screen.getByRole('button', { name: 'b.pdf を削除' }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('shows nothing deletable to a viewer', async () => {
    api.listDocuments.mockResolvedValue([doc('a', 'viewer')]);
    renderList({ canWrite: false, currentUserId: 'viewer' });
    await screen.findByText('a.pdf');
    expect(screen.queryByRole('button', { name: 'a.pdf を削除' })).toBeNull();
  });

  it('deletes after confirmation and asks the caller to refetch the tree', async () => {
    api.listDocuments.mockResolvedValue([doc('mine', 'editor')]);
    api.deleteDocument.mockResolvedValue(undefined);
    const { onDeleted } = renderList();
    fireEvent.click(await screen.findByRole('button', { name: 'mine.pdf を削除' }));

    expect(api.deleteDocument).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '削除する' }));

    await waitFor(() => expect(onDeleted).toHaveBeenCalledTimes(1));
    expect(api.deleteDocument).toHaveBeenCalledWith('mine');
    expect(screen.queryByText('mine.pdf')).toBeNull();
  });

  it('can be cancelled', async () => {
    api.listDocuments.mockResolvedValue([doc('mine', 'editor')]);
    renderList();
    fireEvent.click(await screen.findByRole('button', { name: 'mine.pdf を削除' }));
    fireEvent.click(screen.getByRole('button', { name: 'キャンセル' }));
    expect(screen.queryByRole('button', { name: '削除する' })).toBeNull();
    expect(api.deleteDocument).not.toHaveBeenCalled();
  });

  it('keeps the document and explains why when the server refuses', async () => {
    api.listDocuments.mockResolvedValue([doc('mine', 'editor')]);
    api.deleteDocument.mockRejectedValue(refused('validation', 'FailedPrecondition', 'document is still being uploaded or processed'));
    const { onDeleted } = renderList();
    fireEvent.click(await screen.findByRole('button', { name: 'mine.pdf を削除' }));
    fireEvent.click(screen.getByRole('button', { name: '削除する' }));

    expect(await screen.findByText(/アップロード中か処理中のため/)).toBeTruthy();
    expect(screen.getByText('mine.pdf')).toBeTruthy();
    expect(onDeleted).not.toHaveBeenCalled();
  });

  it('says so when the server refuses for lack of permission', async () => {
    api.listDocuments.mockResolvedValue([doc('mine', 'editor')]);
    api.deleteDocument.mockRejectedValue(refused('auth', 'PermissionDenied', 'forbidden'));
    renderList();
    fireEvent.click(await screen.findByRole('button', { name: 'mine.pdf を削除' }));
    fireEvent.click(screen.getByRole('button', { name: '削除する' }));

    expect(await screen.findByText('このドキュメントを削除する権限がありません。')).toBeTruthy();
  });
});
