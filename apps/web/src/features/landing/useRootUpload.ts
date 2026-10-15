
import { useCallback } from 'react';
import { validateDocumentUploadFile } from '@/features/documents/uploadPolicy';
import type { Workspace } from '@/features/workspaces/api';
import { useCreateWorkspaceMutation } from '@/features/workspaces/mutations';
import type { WorkspacePaperRuntimeState } from '@/features/workspaces/paper/WorkspacePaper';
import { formatConnectErrorMessage, isExpectedConnectError, UserFacingError } from '@/lib/errors';
import { log } from '@/lib/observability/log';

const NEW_WORKSPACE_NAME = '新規ワークスペース';

interface UseRootUploadArgs {
  setPendingRevealNodeId: (id: string | null) => void;
  markWorkspaceVerified: (workspaceId: string) => void;
  handleOpenWorkspace: (workspaceId: string, overrideWorkspace?: Workspace) => Promise<void>;
  uploadWorkspaceFile: (workspaceId: string, file: File) => Promise<{ jobId: string; documentId: string }>;
  injectRuntimeState: (workspaceId: string, state: WorkspacePaperRuntimeState) => void;
}

/**
 * Manages drag-and-drop document upload directly onto the root canvas: creates a fresh workspace,
 * opens it optimistically, and dispatches the file upload and document parsing in the background.
 *
 * @param props - Workspace creation/opening callbacks, runtime-state injection, and upload pipeline.
 * @returns A callback accepting a File and returning after workspace creation/optimistic opening;
 *   the actual upload completion is reflected later through injected runtime state.
 */
export function useRootUpload({
  setPendingRevealNodeId,
  markWorkspaceVerified,
  handleOpenWorkspace,
  uploadWorkspaceFile,
  injectRuntimeState,
}: UseRootUploadArgs) {
  const createMutation = useCreateWorkspaceMutation();

  return useCallback(async (file: File) => {
    // Reject before creating the workspace; otherwise a refused file leaves an
    // empty "新規ワークスペース" behind. RootUploadPaper shows this message.
    const validation = validateDocumentUploadFile(file);
    if (!validation.ok) {
      throw new UserFacingError(validation.error.message);
    }
    const ws = await createMutation.mutateAsync(NEW_WORKSPACE_NAME);
    markWorkspaceVerified(ws.workspaceId);
    await handleOpenWorkspace(ws.workspaceId, ws);
    setPendingRevealNodeId(ws.workspaceId);
    injectRuntimeState(ws.workspaceId, { initialUploading: true, initialUploadMessage: null });
    void (async () => {
      try {
        const uploaded = await uploadWorkspaceFile(ws.workspaceId, file);
        injectRuntimeState(ws.workspaceId, {
          initialActiveJobId: uploaded.jobId,
          initialDocumentId: uploaded.documentId,
          initialUploading: false,
          initialUploadMessage: 'アップロードしました。解析を開始します。',
        });
      } catch (err) {
        log.error('Root upload failed after workspace creation', { source: 'root_upload', workspaceId: ws.workspaceId }, err);
        let message = '通信環境またはファイル形式を確認してください。';
        if (err instanceof UserFacingError) {
          message = err.message;
        } else if (isExpectedConnectError(err)) {
          message = formatConnectErrorMessage(err, message);
        }
        injectRuntimeState(ws.workspaceId, {
          initialUploading: false,
          initialUploadMessage: `アップロードに失敗しました。${message}`,
        });
      }
    })();
  }, [createMutation, handleOpenWorkspace, injectRuntimeState, markWorkspaceVerified, setPendingRevealNodeId, uploadWorkspaceFile]);
}
