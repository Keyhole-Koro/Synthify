
/** Localized message displayed when workspace analysis concludes successfully. */
export const WORKSPACE_COMPLETION_MESSAGE = '解析が完了しました。';
/** Duration in milliseconds before dismissing the completion notification. */
export const WORKSPACE_COMPLETION_MESSAGE_TTL_MS = 4000;
/** Localized message indicating file upload has finished and AI analysis begins. */
export const WORKSPACE_UPLOAD_STARTED_MESSAGE = 'アップロードしました。解析を開始します。';

/**
 * Lifecycle phase of an active workspace session.
 */
export type WorkspaceSessionPhase =
  | 'ready'
  | 'uploading'
  | 'processing'
  | 'recovering'
  | 'failed';

/**
 * State object representing an active workspace user session, tracking
 * jobs, uploads, inline renaming, and lifecycle phase.
 */
export interface WorkspaceSessionState {
  /** Target workspace identifier. */
  workspaceId: string;
  /** Current high-level session lifecycle phase. */
  phase: WorkspaceSessionPhase;
  /** Active and completed job identifiers. */
  job: {
    activeJobId: string | null;
    completedJobIds: Record<string, true>;
    observedSuggestedWorkspaceName: string | null;
  };
  /** File upload status and feedback message. */
  upload: {
    uploading: boolean;
    message: string | null;
  };
  /** Inline workspace renaming state and validation error. */
  rename: {
    editing: boolean;
    draftName: string;
    saving: boolean;
    error: string | null;
  };
}

/**
 * Options used to initialize a new WorkspaceSessionState instance.
 */
export interface CreateWorkspaceSessionStateArgs {
  /** Target workspace identifier. */
  workspaceId: string;
  /** Initial workspace display name. */
  workspaceName: string;
  /** Optional job ID to track upon mounting. */
  initialActiveJobId?: string | null;
  /** Whether an upload is already in flight when mounting. */
  initialUploading?: boolean;
  /** Optional upload status message. */
  initialUploadMessage?: string | null;
}

/**
 * Discriminated union of all actions dispatched to the workspace session reducer.
 */
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

/**
 * Instantiates the default state object for a workspace session.
 *
 * @param args - Initial session arguments.
 * @returns Initialized WorkspaceSessionState.
 */
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
