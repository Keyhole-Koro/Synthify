'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useJobStatus } from '@/features/jobs/useJobStatus';
import { useWorkspaceJobStatuses } from '@/features/jobs/useWorkspaceJobStatuses';
import type { Workspace } from '@/features/workspaces/api';
import { toAppError } from '@/lib/error_normalize';
import type { WorkspacePaperRuntimeState } from './WorkspacePaper';

const COMPLETION_MESSAGE = '解析が完了しました。';
const COMPLETION_MESSAGE_TTL_MS = 4000;

export interface UseWorkspacePaperArgs extends WorkspacePaperRuntimeState {
  workspaceId: string;
  workspaceName: string;
  hasTree: boolean;
  childItemsCount: number;
  onUploadFile: (file: File) => Promise<{ jobId: string; documentId: string }>;
  onRenameWorkspace: (name: string) => Promise<Workspace>;
  onSuggestedWorkspaceName: (name: string) => Promise<void> | void;
  onProcessingComplete?: (jobId: string, createdDocumentRootItemId: string) => Promise<void> | void;
}

// useWorkspacePaper owns everything that is not pure JSX in a workspace card:
// firestore subscriptions for job status, the upload pipeline, rename state,
// and the side effects that bridge between them (active-job recovery on
// reload, completion notification, completion toast auto-dismiss). The View
// (WorkspacePaper.tsx) only receives the resulting state and callbacks.
export function useWorkspacePaper({
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
}: UseWorkspacePaperArgs) {
  const suggestedNameRef = useRef<string | null>(null);
  const completedJobRef = useRef<string | null>(null);

  const [uploading, setUploading] = useState(initialUploading === true);
  const [uploadMessage, setUploadMessage] = useState<string | null>(initialUploadMessage ?? null);
  const [activeJobId, setActiveJobId] = useState<string | null>(initialActiveJobId ?? null);

  const [editingName, setEditingName] = useState(false);
  const [draftName, setDraftName] = useState(workspaceName);
  const [savingName, setSavingName] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);

  const { status: jobStatus, error: jobStatusError } = useJobStatus(workspaceId, activeJobId);
  const { jobs: workspaceJobs } = useWorkspaceJobStatuses(workspaceId);

  // After a reload, in-memory runtime state (initialActiveJobId) is gone, so
  // a job that was still running pre-reload would lose its progress UI. The
  // Firestore subscription is authoritative here: if the workspace has any
  // non-terminal job, adopt the most recent one as the active job so
  // useJobStatus picks up its progress and the completion effect fires.
  useEffect(() => {
    if (activeJobId) return;
    if (workspaceJobs.length === 0) return;
    const inFlight = workspaceJobs.find(
      (job) => job.status === 'queued' || job.status === 'running',
    );
    if (!inFlight) return;
    if (completedJobRef.current === inFlight.jobId) return;
    setActiveJobId(inFlight.jobId);
  }, [activeJobId, workspaceJobs]);

  const isPopulated = hasTree && childItemsCount > 0;
  const isRunning = !!activeJobId && jobStatus?.status === 'running';

  const handleUpload = useCallback(async (file: File) => {
    setUploading(true);
    setUploadMessage(null);
    try {
      const job = await onUploadFile(file);
      setActiveJobId(job.jobId);
      completedJobRef.current = null;
      suggestedNameRef.current = null;
      setUploadMessage('アップロードしました。解析を開始します。');
    } catch (err) {
      console.error('Upload failed:', err);
      const appErr = toAppError(err);
      setUploadMessage(appErr.message);
    } finally {
      setUploading(false);
    }
  }, [onUploadFile]);

  useEffect(() => {
    if (!jobStatus || !activeJobId) return;
    if (jobStatus.suggestedWorkspaceName && suggestedNameRef.current !== jobStatus.suggestedWorkspaceName) {
      suggestedNameRef.current = jobStatus.suggestedWorkspaceName;
      void onSuggestedWorkspaceName(jobStatus.suggestedWorkspaceName);
    }
    if (jobStatus.status !== 'succeeded') return;
    // The worker writes createdDocumentRootItemId atomically with
    // status=succeeded, so its absence here means an in-flight schema
    // mismatch — bail out rather than feed undefined into the tree merge.
    const createdDocumentRootItemId = jobStatus.createdDocumentRootItemId;
    if (!createdDocumentRootItemId) return;
    if (completedJobRef.current === activeJobId) return;
    completedJobRef.current = activeJobId;
    setUploadMessage(COMPLETION_MESSAGE);
    void onProcessingComplete?.(activeJobId, createdDocumentRootItemId);
  }, [activeJobId, jobStatus, onProcessingComplete, onSuggestedWorkspaceName]);

  // Auto-dismiss the success toast after a short window. Failure / upload
  // messages are unrelated strings so they stay until the user does something
  // that overwrites them. Comparing the rendered text keeps this side-effect
  // local without threading another state flag.
  useEffect(() => {
    if (uploadMessage !== COMPLETION_MESSAGE) return;
    const timer = window.setTimeout(() => {
      setUploadMessage((current) => (current === COMPLETION_MESSAGE ? null : current));
    }, COMPLETION_MESSAGE_TTL_MS);
    return () => window.clearTimeout(timer);
  }, [uploadMessage]);

  const commitName = useCallback(async () => {
    const nextName = draftName.trim();
    if (!nextName || nextName === workspaceName) {
      setEditingName(false);
      setDraftName(workspaceName);
      return;
    }
    setSavingName(true);
    setNameError(null);
    try {
      await onRenameWorkspace(nextName);
      setEditingName(false);
    } catch (err) {
      console.error('Rename workspace failed:', err);
      const appErr = toAppError(err);
      setNameError(appErr.message);
    } finally {
      setSavingName(false);
    }
  }, [draftName, onRenameWorkspace, workspaceName]);

  const startRename = useCallback(() => {
    setDraftName(workspaceName);
    setEditingName(true);
  }, [workspaceName]);

  const cancelRename = useCallback(() => {
    setEditingName(false);
    setDraftName(workspaceName);
  }, [workspaceName]);

  const isJustCompleted = uploadMessage === COMPLETION_MESSAGE;

  return {
    // job
    jobStatus,
    jobStatusError,
    workspaceJobs,
    activeJobId,
    isRunning,
    // upload
    uploading,
    uploadMessage,
    isJustCompleted,
    handleUpload,
    // rename
    editingName,
    draftName,
    savingName,
    nameError,
    setDraftName,
    commitName,
    startRename,
    cancelRename,
    // layout
    isPopulated,
  };
}
