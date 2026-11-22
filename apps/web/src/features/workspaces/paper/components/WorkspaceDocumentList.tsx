import React, { useCallback, useEffect, useState } from 'react';
import {
  deleteDocument,
  DocumentLifecycleState,
  listDocuments,
  type Document,
} from '@/features/documents/api';
import { InlineError } from '@/components/error/InlineError';
import { toAppError } from '@/lib/error_normalize';

interface WorkspaceDocumentListProps {
  workspaceId: string;
  currentUserId: string | null;
  /** Owners may delete every document. */
  canManage: boolean;
  /** Editors may delete the documents they uploaded. */
  canWrite: boolean;
  /** Changes whenever an upload or a job starts or ends, to refetch the list. */
  refreshKey: string;
  /** Called after a deletion so the caller can refetch the tree. */
  onDeleted: () => void;
  className?: string;
}

// WorkspaceDocumentList lists the workspace's documents with a delete action.
// Deleting asks for confirmation inline, the same two-step pattern as the
// workspace delete button.
export function WorkspaceDocumentList({
  workspaceId,
  currentUserId,
  canManage,
  canWrite,
  refreshKey,
  onDeleted,
  className = '',
}: WorkspaceDocumentListProps) {
  const [documents, setDocuments] = useState<Document[]>([]);
  const [confirmingId, setConfirmingId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    listDocuments(workspaceId)
      .then((docs) => {
        if (!cancelled) setDocuments(docs);
      })
      .catch(() => {
        // The list is secondary to the tree; a failed fetch just hides it.
        if (!cancelled) setDocuments([]);
      });
    return () => {
      cancelled = true;
    };
  }, [workspaceId, refreshKey]);

  const canDelete = useCallback(
    (doc: Document) => canManage || (canWrite && currentUserId !== null && doc.uploadedBy === currentUserId),
    [canManage, canWrite, currentUserId],
  );

  const handleDelete = useCallback(async (doc: Document) => {
    setDeletingId(doc.documentId);
    setError(null);
    try {
      await deleteDocument(doc.documentId);
      setDocuments((current) => current.filter((d) => d.documentId !== doc.documentId));
      setConfirmingId(null);
      onDeleted();
    } catch (err) {
      setError(deleteErrorMessage(err));
    } finally {
      setDeletingId(null);
    }
  }, [onDeleted]);

  if (documents.length === 0) {
    return null;
  }

  return (
    <div className={className} data-testid="workspace-document-list">
      <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-[0.18em] text-stone-400">
        ドキュメント
      </p>
      <ul className="divide-y divide-stone-100 overflow-hidden rounded-lg border border-stone-100 bg-white">
        {documents.map((doc) => {
          const processing = doc.status === DocumentLifecycleState.PROCESSING;
          const confirming = confirmingId === doc.documentId;
          const deleting = deletingId === doc.documentId;
          return (
            <li key={doc.documentId} className="px-3 py-2 text-[11px]">
              <div className="flex items-center gap-3">
                <span className="flex-1 truncate text-stone-700" title={doc.filename}>
                  {doc.filename}
                </span>
                {processing && <span className="text-[10px] text-stone-400">処理中</span>}
                {canDelete(doc) && !confirming && (
                  <button
                    type="button"
                    disabled={processing || deletingId !== null}
                    onClick={() => {
                      setError(null);
                      setConfirmingId(doc.documentId);
                    }}
                    className="h-6 rounded-md border border-stone-200 bg-white px-2 text-[11px] text-stone-400 hover:border-red-200 hover:text-red-500 disabled:opacity-50"
                    aria-label={`${doc.filename} を削除`}
                  >
                    削除
                  </button>
                )}
              </div>
              {confirming && (
                <div className="mt-2 rounded-md bg-red-50 p-2" role="group" aria-label={`${doc.filename} の削除の確認`}>
                  <p className="text-[11px] leading-relaxed text-red-700">
                    「{doc.filename}」を削除します。原本と抽出結果、このドキュメントだけから作られたノードも消え、元に戻せません。
                  </p>
                  <div className="mt-2 flex justify-end gap-2">
                    <button
                      type="button"
                      disabled={deleting}
                      onClick={() => setConfirmingId(null)}
                      className="h-6 rounded-md border border-stone-200 bg-white px-2 text-[11px] text-stone-500 disabled:opacity-60"
                    >
                      キャンセル
                    </button>
                    <button
                      type="button"
                      disabled={deleting}
                      onClick={() => void handleDelete(doc)}
                      className="h-6 rounded-md border border-red-200 bg-white px-2 text-[11px] font-medium text-red-600 disabled:opacity-60"
                    >
                      {deleting ? '削除中…' : '削除する'}
                    </button>
                  </div>
                </div>
              )}
            </li>
          );
        })}
      </ul>
      {error && <InlineError message={error} className="mt-2" />}
    </div>
  );
}

function deleteErrorMessage(err: unknown): string {
  const { code } = toAppError(err);
  if (code === 'FailedPrecondition') {
    return 'アップロード中か処理中のため、まだ削除できません。終わってからもう一度試してください。';
  }
  if (code === 'PermissionDenied') {
    return 'このドキュメントを削除する権限がありません。';
  }
  return 'ドキュメントの削除に失敗しました。時間をおいてもう一度試してください。';
}
