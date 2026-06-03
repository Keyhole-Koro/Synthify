import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';

export const WORKSPACE_COMPLETION_MESSAGE = '解析が完了しました。';
export const WORKSPACE_COMPLETION_MESSAGE_TTL_MS = 4000;
export const WORKSPACE_UPLOAD_STARTED_MESSAGE = 'アップロードしました。解析を開始します。';

export type WorkspaceSessionPhase =
  | 'ready'
  | 'uploading'
  | 'processing'
  | 'recovering'
  | 'failed';

export interface WorkspaceSessionState {
  workspaceId: string;
  phase: WorkspaceSessionPhase;
  job: {
    activeJobId: string | null;
    completedJobIds: Record<string, true>;
    observedSuggestedWorkspaceName: string | null;
  };
  upload: {
    uploading: boolean;
    message: string | null;
  };
  rename: {
    editing: boolean;
    draftName: string;
    saving: boolean;
    error: string | null;
  };
}

export interface CreateWorkspaceSessionStateArgs {
  workspaceId: string;
  workspaceName: string;
  initialActiveJobId?: string | null;
  initialUploading?: boolean;
  initialUploadMessage?: string | null;
}

export type WorkspaceSessionAction =
  | { type: 'workspace/propsSynced'; workspaceName: string }
  | { type: 'job/adopted'; jobId: string; reason: 'inFlight' | 'completedTreeChange' }
  | { type: 'job/suggestedWorkspaceNameObserved'; suggestedWorkspaceName: string }
  | { type: 'job/succeeded'; jobId: string }
  | { type: 'upload/started' }
  | { type: 'upload/enqueued'; jobId: string; documentId: string }
  | { type: 'upload/failed'; message: string }
  | { type: 'upload/messageDismissed' }
  | { type: 'rename/started'; currentName: string }
  | { type: 'rename/inputChanged'; value: string }
  | { type: 'rename/saveStarted' }
  | { type: 'rename/saveSucceeded'; name: string }
  | { type: 'rename/saveFailed'; message: string }
  | { type: 'rename/cancelled'; currentName: string };

export interface WorkspaceSessionSnapshot {
  state: WorkspaceSessionState;
  jobStatus: FirestoreJobStatus | undefined;
}

export function createWorkspaceSessionState({
  workspaceId,
  workspaceName,
  initialActiveJobId,
  initialUploading,
  initialUploadMessage,
}: CreateWorkspaceSessionStateArgs): WorkspaceSessionState {
  return {
    workspaceId,
    phase: initialUploading ? 'uploading' : initialActiveJobId ? 'processing' : 'ready',
    job: {
      activeJobId: initialActiveJobId ?? null,
      completedJobIds: {},
      observedSuggestedWorkspaceName: null,
    },
    upload: {
      uploading: initialUploading === true,
      message: initialUploadMessage ?? null,
    },
    rename: {
      editing: false,
      draftName: workspaceName,
      saving: false,
      error: null,
    },
  };
}
