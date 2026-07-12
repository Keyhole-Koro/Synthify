'use client';

import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { isJobInFlight } from '@/features/jobs/contracts/jobStatusContract';
import { useJobStatus } from '@/features/jobs/firestore/useJobStatus';
import { useWorkspaceJobStatuses } from '@/features/jobs/firestore/useWorkspaceJobStatuses';
import type { Workspace } from '@/features/workspaces/api';
import { toAppError } from '@/lib/error_normalize';
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

export interface UseWorkspaceSessionArgs extends WorkspacePaperRuntimeState {
  workspaceId: string;
  workspaceName: string;
  hasTree: boolean;
  childItemsCount: number;
  onUploadFile: (file: File) => Promise<{ jobId: string; documentId: string }>;
  onRenameWorkspace: (name: string) => Promise<Workspace>;
  onSuggestedWorkspaceName: (name: string) => Promise<void> | void;
  onProcessingComplete?: (jobId: string) => Promise<void> | void;
}

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
      const appErr = toAppError(err);
      dispatch({ type: 'upload/failed', message: appErr.message });
    }
  }, [onUploadFile, workspaceId]);

  const commitName = useCallback(async () => {
    const nextName = state.rename.draftName.trim();
    if (!nextName) {
      dispatch({ type: 'rename/saveFailed', message: 'ワークスペース名を入力してください。' });
      return;
    }
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
      const appErr = toAppError(err);
      dispatch({ type: 'rename/saveFailed', message: appErr.message });
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
