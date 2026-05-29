'use client';

import { useCallback } from 'react';
import { createWorkspace, type Workspace } from '@/features/workspaces/api';
import type { WorkspacePaperRuntimeState } from '@/features/workspaces/WorkspacePaper';
import { toAppError } from '@/lib/error_normalize';

const NEW_WORKSPACE_NAME = '新規ワークスペース';

interface UseRootUploadArgs {
  setWorkspaces: React.Dispatch<React.SetStateAction<Workspace[]>>;
  setPendingRevealNodeId: (id: string | null) => void;
  handleOpenWorkspace: (workspaceId: string, overrideWorkspace?: Workspace) => Promise<void>;
  uploadWorkspaceFile: (workspaceId: string, file: File) => Promise<{ jobId: string; documentId: string }>;
  injectRuntimeState: (workspaceId: string, state: WorkspacePaperRuntimeState) => void;
}

// useRootUpload handles the "drop a file onto the workspaces area" flow:
// create a fresh workspace, open it optimistically, then run the upload in
// the background and stream its result through runtime state so the
// workspace's progress UI lights up. Errors are surfaced through the same
// runtime-state channel.
export function useRootUpload({
  setWorkspaces,
  setPendingRevealNodeId,
  handleOpenWorkspace,
  uploadWorkspaceFile,
  injectRuntimeState,
}: UseRootUploadArgs) {
  return useCallback(async (file: File) => {
    const ws = await createWorkspace(NEW_WORKSPACE_NAME);
    setWorkspaces((prev) => [ws, ...prev]);
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
        console.error('Root upload failed after workspace creation:', err);
        const appError = toAppError(err);
        injectRuntimeState(ws.workspaceId, {
          initialUploading: false,
          initialUploadMessage: `アップロードに失敗しました。${appError.message}`,
        });
      }
    })();
  }, [handleOpenWorkspace, injectRuntimeState, setPendingRevealNodeId, setWorkspaces, uploadWorkspaceFile]);
}
