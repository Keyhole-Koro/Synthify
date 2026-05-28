'use client';

import React, { useRef, useState, useCallback, useEffect } from 'react';
import { useJobStatus } from '@/features/jobs/useJobStatus';
import { useWorkspaceJobStatuses } from '@/features/jobs/useWorkspaceJobStatuses';
import { WorkspaceHeader } from './components/WorkspaceHeader';
import { WorkspaceDropzone } from './components/WorkspaceDropzone';
import { WorkspaceJobProgress } from './components/WorkspaceJobProgress';
import { WorkspaceJobList } from './components/WorkspaceJobList';
import { type Workspace } from '@/features/workspaces/api';
import { InlineError } from '@/components/error/InlineError';
import { toAppError } from '@/lib/error_normalize';

export interface WorkspacePaperRuntimeState {
  initialActiveJobId?: string | null;
  initialDocumentId?: string | null;
  initialUploading?: boolean;
  initialUploadMessage?: string | null;
}

interface WorkspacePaperProps extends WorkspacePaperRuntimeState {
  workspace: Workspace;
  workspaceId: string;
  workspaceName: string;
  hasTree: boolean;
  childItems: { id: string }[];
  onUploadFile: (file: File) => Promise<{ jobId: string; documentId: string }>;
  onRenameWorkspace: (name: string) => Promise<Workspace>;
  onSuggestedWorkspaceName: (name: string) => Promise<void> | void;
  onProcessingComplete?: (jobId: string) => Promise<void> | void;
}

export function WorkspacePaper({
  workspaceId,
  workspaceName,
  hasTree,
  childItems,
  initialActiveJobId,
  initialUploading,
  initialUploadMessage,
  onUploadFile,
  onRenameWorkspace,
  onSuggestedWorkspaceName,
  onProcessingComplete,
}: WorkspacePaperProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const suggestedNameRef = useRef<string | null>(null);
  const [uploading, setUploading] = useState(initialUploading === true);
  const [uploadMessage, setUploadMessage] = useState<string | null>(initialUploadMessage ?? null);
  const [activeJobId, setActiveJobId] = useState<string | null>(initialActiveJobId ?? null);
  const completedJobRef = useRef<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [isHovered, setIsHovered] = useState(false);
  const [isPinned, setIsPinned] = useState(false);
  const [editingName, setEditingName] = useState(false);
  const [draftName, setDraftName] = useState(workspaceName);
  const [savingName, setSavingName] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const { status: jobStatus, error: jobStatusError } = useJobStatus(workspaceId, activeJobId);
  const { jobs: workspaceJobs } = useWorkspaceJobStatuses(workspaceId);

  const isTreeMissing = !hasTree;
  const isPopulated = hasTree && childItems.length > 0;
  const isExpanded = !isPopulated || isHovered || isPinned;
  const isRunning = !!activeJobId && jobStatus?.status === 'running';

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

  const handleUpload = async (file: File) => {
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
  };

  useEffect(() => {
    if (!jobStatus || !activeJobId) return;
    if (jobStatus.suggestedWorkspaceName && suggestedNameRef.current !== jobStatus.suggestedWorkspaceName) {
      suggestedNameRef.current = jobStatus.suggestedWorkspaceName;
      void onSuggestedWorkspaceName(jobStatus.suggestedWorkspaceName);
    }
    if (jobStatus.status !== 'succeeded') return;
    if (completedJobRef.current === activeJobId) return;
    completedJobRef.current = activeJobId;
    setUploadMessage('解析が完了しました。');
    void onProcessingComplete?.(activeJobId);
  }, [activeJobId, jobStatus, onProcessingComplete, onSuggestedWorkspaceName]);

  const handleMouseEnter = useCallback(() => setIsHovered(true), []);
  const handleMouseLeave = useCallback(() => {
    setIsHovered(false);
    if (uploadMessage === '解析が完了しました。') setUploadMessage(null);
  }, [uploadMessage]);

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    await handleUpload(file);
  };

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    if (!e.currentTarget.contains(e.relatedTarget as Node)) {
      setIsDragging(false);
    }
  }, []);

  const handleDrop = useCallback(async (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const file = e.dataTransfer.files[0];
    if (!file) return;
    await handleUpload(file);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onUploadFile]);

  return (
    <div
      className="flex h-full flex-col"
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <input
        ref={fileInputRef}
        type="file"
        className="sr-only"
        onChange={handleFileChange}
        onClick={(e) => {
          (e.target as HTMLInputElement).value = '';
        }}
      />

      {/* Compact header (populated mode only) */}
      {isPopulated && (
        <>
          <WorkspaceHeader
            workspaceName={workspaceName}
            draftName={draftName}
            editingName={editingName}
            savingName={savingName}
            childItemsCount={childItems.length}
            isRunning={isRunning}
            jobProgress={jobStatus?.progress}
            isJustCompleted={uploadMessage === '解析が完了しました。'}
            isPinned={isPinned}
            onTogglePinned={() => setIsPinned((p) => !p)}
            onStartRename={() => {
              setDraftName(workspaceName);
              setEditingName(true);
            }}
            onDraftNameChange={setDraftName}
            onCommitName={commitName}
            onCancelRename={() => {
              setEditingName(false);
              setDraftName(workspaceName);
            }}
          />
          {nameError && <InlineError message={nameError} className="-mt-2 px-5 pb-2" />}
        </>
      )}

      {/* Expanded content */}
      {isExpanded && (
        <div className={['flex flex-col gap-5 overflow-y-auto', isPopulated ? 'px-5 pb-5' : 'flex-1 p-5'].join(' ')}>
          {/* Header (empty mode) */}
          {!isPopulated && (
            <div>
              <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-indigo-400/80">Workspace</p>
              {editingName ? (
                <form
                  className="mt-1 flex items-center gap-2"
                  onSubmit={(e) => {
                    e.preventDefault();
                    void commitName();
                  }}
                >
                  <input
                    value={draftName}
                    maxLength={64}
                    disabled={savingName}
                    onChange={(e) => setDraftName(e.target.value)}
                    onBlur={() => void commitName()}
                    autoFocus
                    className="min-w-0 flex-1 rounded-md border border-indigo-200 bg-white px-2 py-1 text-lg font-semibold tracking-tight text-stone-800 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 disabled:opacity-60"
                  />
                  <button
                    type="button"
                    disabled={savingName}
                    onClick={() => {
                      setEditingName(false);
                      setDraftName(workspaceName);
                    }}
                    className="rounded-md border border-stone-200 px-2 py-1 text-[11px] text-stone-500 disabled:opacity-60"
                  >
                    取消
                  </button>
                </form>
              ) : (
                <button
                  type="button"
                  onClick={() => {
                    setDraftName(workspaceName);
                    setEditingName(true);
                  }}
                  className="mt-0.5 block max-w-full truncate text-left text-lg font-semibold tracking-tight text-stone-800 hover:text-indigo-500"
                >
                  {workspaceName}
                </button>
              )}
              {nameError && <InlineError message={nameError} className="mt-1" />}
            </div>
          )}

          {/* Upload zone / add button */}
          <div className={!isPopulated ? 'flex flex-1 flex-col justify-center' : ''}>

            <WorkspaceDropzone
              isTreeMissing={isTreeMissing}
              hasChildItems={childItems.length > 0}
              uploading={uploading}
              activeJobId={activeJobId}
              isDragging={isDragging}
              jobStatusMessage={jobStatusError?.message ?? jobStatus?.message}
              jobStatusProgress={jobStatus?.progress}
              jobStatusFailed={jobStatus?.status === 'failed'}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
            />

            {uploadMessage && (
              <div className="mt-2.5 text-center">
                {uploadMessage.includes('失敗') || uploadMessage.includes('エラー') ? (
                  <InlineError message={uploadMessage} className="justify-center" />
                ) : (
                  <p className="text-[11px] text-indigo-500">
                    {uploadMessage}
                  </p>
                )}
              </div>
            )}

            {/* Progress: populated mode only (empty mode shows it inline inside the drop zone) */}
            {isPopulated && (jobStatus || jobStatusError) && (
              <WorkspaceJobProgress
                message={jobStatusError?.message ?? jobStatus?.message}
                progress={jobStatus?.progress}
                isFailed={jobStatus?.status === 'failed'}
              />
            )}

            <WorkspaceJobList workspaceJobs={workspaceJobs} />
          </div>
        </div>
      )}
    </div>
  );
}
