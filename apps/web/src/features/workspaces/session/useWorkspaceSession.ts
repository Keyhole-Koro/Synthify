
import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { isJobInFlight } from '@/features/jobs/contracts/jobStatusContract';
import { useJobStatus } from '@/features/jobs/firestore/useJobStatus';
import { useWorkspaceJobStatuses } from '@/features/jobs/firestore/useWorkspaceJobStatuses';
import type { Workspace } from '@/features/workspaces/api';
import { validateWorkspaceName } from '@/features/workspaces/workspaceValidation';
import { ConnectError, Code } from '@connectrpc/connect';
import { formatConnectErrorMessage, isExpectedConnectError, UserFacingError } from '@/lib/errors';
import { log } from '@/lib/observability/log';
import type { WorkspacePaperRuntimeState } from '../paper/WorkspacePaper';
import { selectRecoverableWorkspaceJob } from './workspaceRecovery';
import { workspaceSessionReducer } from './workspaceSessionReducer';
import {
  createWorkspaceSessionState,
  WORKSPACE_COMPLETION_MESSAGE,
  WORKSPACE_COMPLETION_MESSAGE_TTL_MS,
  type CreateWorkspaceSessionStateArgs,
} from './workspaceSessionTypes';
import {
  didJobChangeTree,
  isWorkspaceJobFailed,
  isWorkspaceJobRunning,
  isWorkspaceJustCompleted,
} from './workspaceSessionSelectors';

/**
 * Arguments for the {@link useWorkspaceSession} state management hook.
 */
export interface UseWorkspaceSessionArgs extends WorkspacePaperRuntimeState {
  /** Target workspace identifier. */
  workspaceId: string;
  /** Current display name of the workspace. */
  workspaceName: string;
  /** Whether the workspace has a tree loaded. */
  hasTree: boolean;
  /** Number of child items currently present under the workspace. */
  childItemsCount: number;
  /** Callback to initiate document upload pipeline. */
  onUploadFile: (file: File) => Promise<{ jobId: string; documentId: string }>;
  /** Callback to persist updated workspace display name. */
  onRenameWorkspace: (name: string) => Promise<Workspace>;
  /** Callback invoked when an AI-suggested title is detected. */
  onSuggestedWorkspaceName: (name: string) => Promise<void> | void;
  /** Optional callback fired when an ongoing processing job completes. */
  onProcessingComplete?: (jobId: string) => Promise<void> | void;
}

/**
 * Manages the client-side session state for an active workspace paper, coordinating
 * uploads, job status polling/recovery, title renaming, and completion notifications.
 *
 * @param args - Initial session configuration and event callbacks.
 * @returns State and updater handlers for driving the workspace paper UI.
 */
export function useWorkspaceSession({
  workspaceId,
  workspaceName,
  hasTree,
  childItemsCount,
  initialActiveJobId,
  initialUploading,
  initialUploadMessage,
  onUploadFile,
  onRenameWorkspace,
  onSuggestedWorkspaceName,
  onProcessingComplete,
}: UseWorkspaceSessionArgs) {
  const initialStateArgs = useMemo<CreateWorkspaceSessionStateArgs>(() => ({
    workspaceId,
    workspaceName,
    initialActiveJobId,
    initialUploading,
    initialUploadMessage,
  }), [initialActiveJobId, initialUploadMessage, initialUploading, workspaceId, workspaceName]);

  const [state, dispatch] = useReducer(
    workspaceSessionReducer,
    initialStateArgs,
    createWorkspaceSessionState,
  );

  const activeJobId = state.job.activeJobId;
  const completedJobIds = state.job.completedJobIds;
  const observedSuggestedWorkspaceName = state.job.observedSuggestedWorkspaceName;
  const uploadMessage = state.upload.message;
  const { status: rawJobStatus, error: jobStatusError } = useJobStatus(workspaceId, activeJobId);
  const { jobs: workspaceJobs } = useWorkspaceJobStatuses(workspaceId);

  useEffect(() => {
    dispatch({ type: 'workspace/propsSynced', workspaceName });
  }, [workspaceName]);

  useEffect(() => {
    const recoverableJob = selectRecoverableWorkspaceJob(workspaceJobs, {
      hasActiveJob: !!activeJobId,
      hasTree,
      childItemsCount,
      completedJobIds,
    });
    if (!recoverableJob) return;
    dispatch({
      type: 'job/adopted',
      jobId: recoverableJob.jobId,
      reason: isJobInFlight(recoverableJob) ? 'inFlight' : 'completedTreeChange',
    });
  }, [activeJobId, childItemsCount, completedJobIds, hasTree, workspaceJobs]);

  // useJobStatus subscribes to a single job doc once activeJobId is set,
  // but its first emission is null while the snapshot is still in flight.
  // useWorkspaceJobStatuses is already streaming the same workspace's job
  // collection, so use its copy as a fallback so the progress UI does not
  // blink off between "activeJobId resolved" and "useJobStatus's first
  // snapshot arrived".
  const jobStatus = rawJobStatus
    ?? (activeJobId ? workspaceJobs.find((job) => job.jobId === activeJobId) : undefined);

  useEffect(() => {
    if (!jobStatus || !activeJobId) return;
    const suggestedWorkspaceName = jobStatus.suggestedWorkspaceName;
    if (suggestedWorkspaceName && observedSuggestedWorkspaceName !== suggestedWorkspaceName) {
      dispatch({ type: 'job/suggestedWorkspaceNameObserved', suggestedWorkspaceName });
      void onSuggestedWorkspaceName(suggestedWorkspaceName);
    }

    if (!didJobChangeTree(jobStatus)) return;
    if (completedJobIds[activeJobId] === true) return;
    dispatch({ type: 'job/succeeded', jobId: activeJobId });
    void onProcessingComplete?.(activeJobId);
  }, [activeJobId, completedJobIds, jobStatus, observedSuggestedWorkspaceName, onProcessingComplete, onSuggestedWorkspaceName]);

  useEffect(() => {
    if (uploadMessage !== WORKSPACE_COMPLETION_MESSAGE) return;
    const timer = window.setTimeout(() => {
      dispatch({ type: 'upload/messageDismissed' });
    }, WORKSPACE_COMPLETION_MESSAGE_TTL_MS);
    return () => window.clearTimeout(timer);
  }, [uploadMessage]);

  const handleUpload = useCallback(async (file: File) => {
    dispatch({ type: 'upload/started' });
    try {
      const job = await onUploadFile(file);
      dispatch({ type: 'upload/enqueued', jobId: job.jobId, documentId: job.documentId });
    } catch (err) {
      log.error('Upload failed', { source: 'workspace_upload', workspaceId }, err);
      if (err instanceof UserFacingError) {
        dispatch({ type: 'upload/failed', message: err.message });
        return;
      }
      if (err instanceof ConnectError) {
        if (err.code === Code.PermissionDenied) {
          dispatch({ type: 'upload/failed', message: 'アップロードする権限がありません。' });
          return;
        }
        if (err.code === Code.ResourceExhausted) {
          dispatch({ type: 'upload/failed', message: err.rawMessage || 'ストレージ容量またはクォータの上限に達しました。' });
          return;
        }
        if (err.code === Code.InvalidArgument) {
          dispatch({ type: 'upload/failed', message: err.rawMessage || 'サポートされていないファイル形式です。' });
          return;
        }
        if (err.code === Code.Unavailable || err.code === Code.DeadlineExceeded) {
          dispatch({ type: 'upload/failed', message: '通信環境を確認して、再度お試しください。' });
          return;
        }
        if (isExpectedConnectError(err)) {
          dispatch({ type: 'upload/failed', message: formatConnectErrorMessage(err, 'アップロードに失敗しました。') });
          return;
        }
      }
      dispatch({ type: 'upload/failed', message: 'アップロードに失敗しました。' });
    }
  }, [onUploadFile, workspaceId]);

  const commitName = useCallback(async () => {
    const validated = validateWorkspaceName(state.rename.draftName);
    if (!validated.ok) {
      dispatch({ type: 'rename/saveFailed', message: validated.error.message });
      return;
    }
    const nextName = validated.value;
    if (nextName === workspaceName) {
      dispatch({ type: 'rename/cancelled', currentName: workspaceName });
      return;
    }
    dispatch({ type: 'rename/saveStarted' });
    try {
      await onRenameWorkspace(nextName);
      dispatch({ type: 'rename/saveSucceeded', name: nextName });
    } catch (err) {
      log.error('Rename workspace failed', { source: 'workspace_rename', workspaceId }, err);
      if (err instanceof UserFacingError) {
        dispatch({ type: 'rename/saveFailed', message: err.message });
        return;
      }
      if (err instanceof ConnectError) {
        if (err.code === Code.PermissionDenied) {
          dispatch({ type: 'rename/saveFailed', message: 'ワークスペース名を変更する権限がありません。' });
          return;
        }
        if (err.code === Code.NotFound) {
          dispatch({ type: 'rename/saveFailed', message: 'ワークスペースが見つかりませんでした。' });
          return;
        }
        if (err.code === Code.InvalidArgument) {
          dispatch({ type: 'rename/saveFailed', message: err.rawMessage || 'ワークスペース名が不正です。' });
          return;
        }
        if (err.code === Code.Unavailable || err.code === Code.DeadlineExceeded) {
          dispatch({ type: 'rename/saveFailed', message: '通信環境を確認して、再度お試しください。' });
          return;
        }
        if (isExpectedConnectError(err)) {
          dispatch({
            type: 'rename/saveFailed',
            message: formatConnectErrorMessage(err, 'ワークスペース名の変更に失敗しました。'),
          });
          return;
        }
      }
      dispatch({ type: 'rename/saveFailed', message: 'ワークスペース名の変更に失敗しました。' });
    }
  }, [onRenameWorkspace, state.rename.draftName, workspaceId, workspaceName]);

  const startRename = useCallback(() => {
    dispatch({ type: 'rename/started', currentName: workspaceName });
  }, [workspaceName]);

  const cancelRename = useCallback(() => {
    dispatch({ type: 'rename/cancelled', currentName: workspaceName });
  }, [workspaceName]);

  const setDraftName = useCallback((value: string) => {
    dispatch({ type: 'rename/inputChanged', value });
  }, []);

  return {
    state,
    jobStatus,
    jobStatusError,
    workspaceJobs,
    activeJobId,
    isRunning: isWorkspaceJobRunning(state, jobStatus),
    isFailed: isWorkspaceJobFailed(jobStatus),
    uploading: state.upload.uploading,
    uploadMessage: state.upload.message,
    isJustCompleted: isWorkspaceJustCompleted(state),
    handleUpload,
    editingName: state.rename.editing,
    draftName: state.rename.draftName,
    savingName: state.rename.saving,
    nameError: state.rename.error,
    setDraftName,
    commitName,
    startRename,
    cancelRename,
    isPopulated: hasTree && childItemsCount > 0,
  };
}
