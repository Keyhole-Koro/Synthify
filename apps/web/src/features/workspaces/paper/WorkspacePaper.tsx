'use client';

import React, { useCallback, useRef, useState } from 'react';
import { isJobFailed, isJobSucceeded, isJobTerminal } from '@/features/jobs/contracts/jobStatusContract';
import { WorkspaceHeader } from './components/WorkspaceHeader';
import { WorkspaceDropzone } from './components/WorkspaceDropzone';
import { WorkspaceJobProgress } from './components/WorkspaceJobProgress';
import { WorkspaceJobList } from './components/WorkspaceJobList';
import { WorkspaceDocumentList } from './components/WorkspaceDocumentList';
import { WorkspaceEmptyHeader } from './components/WorkspaceEmptyHeader';
import { type Workspace } from '@/features/workspaces/api';
import { InlineError } from '@/components/error/InlineError';
import { useWorkspaceSession } from '../session/useWorkspaceSession';

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
  childItems: { id: string; title: string }[];
  onUploadFile: (file: File) => Promise<{ jobId: string; documentId: string }>;
  onRenameWorkspace: (name: string) => Promise<Workspace>;
  onSuggestedWorkspaceName: (name: string) => Promise<void> | void;
  onProcessingComplete?: (jobId: string) => Promise<void> | void;
}

export function WorkspacePaper(props: WorkspacePaperProps) {
  const {
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
  } = props;

  const {
    jobStatus,
    jobStatusError,
    workspaceJobs,
    activeJobId,
    isRunning,
    isFailed,
    uploading,
    uploadMessage,
    isJustCompleted,
    isPopulated,
    handleUpload,
    editingName,
    draftName,
    savingName,
    nameError,
    setDraftName,
    commitName,
    startRename,
    cancelRename,
  } = useWorkspaceSession({
    workspaceId,
    workspaceName,
    hasTree,
    childItemsCount: childItems.length,
    initialActiveJobId,
    initialUploading,
    initialUploadMessage,
    onUploadFile,
    onRenameWorkspace,
    onSuggestedWorkspaceName,
    onProcessingComplete,
  });

  const fileInputRef = useRef<HTMLInputElement>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [isHovered, setIsHovered] = useState(false);
  const [isPinned, setIsPinned] = useState(false);

  const isTreeMissing = !hasTree;
  const isExpanded = !isPopulated || isHovered || isPinned || isFailed;

  const handleMouseEnter = useCallback(() => setIsHovered(true), []);
  const handleMouseLeave = useCallback(() => setIsHovered(false), []);

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
  }, [handleUpload]);

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
            isFailed={isFailed}
            jobProgress={jobStatus?.progress}
            isJustCompleted={isJustCompleted}
            isPinned={isPinned}
            onTogglePinned={() => setIsPinned((p) => !p)}
            onStartRename={startRename}
            onDraftNameChange={setDraftName}
            onCommitName={commitName}
            onCancelRename={cancelRename}
          />
          {nameError && <InlineError message={nameError} className="-mt-2 px-5 pb-2" />}

          {/* Always-visible section: document cards + recent jobs. Shown
              regardless of hover/pin so the workspace's contents and job
              activity are visible at a glance. The cards carry data-paper-id
              so clicking opens the document_root child paper as before. */}
          <div className="flex flex-col gap-4 px-5 pb-4">
            <WorkspaceDocumentList documents={childItems} />
            <WorkspaceJobList workspaceJobs={workspaceJobs} />
          </div>
        </>
      )}

      {/* Expanded content */}
      {isExpanded && (
        <div className={['flex flex-col gap-5 overflow-y-auto', isPopulated ? 'px-5 pb-5' : 'flex-1 p-5'].join(' ')}>
          {!isPopulated && (
            <WorkspaceEmptyHeader
              workspaceName={workspaceName}
              editingName={editingName}
              draftName={draftName}
              savingName={savingName}
              nameError={nameError}
              onDraftNameChange={setDraftName}
              onStartRename={startRename}
              onCommitName={commitName}
              onCancelRename={cancelRename}
            />
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
              jobStatusFailed={isJobFailed(jobStatus)}
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

            {/* Progress: populated mode only (empty mode shows it inline inside the drop zone).
                Hide once succeeded so the bar disappears after completion; the
                completion toast / Recent jobs list takes over from there. */}
            {isPopulated && (jobStatusError || (jobStatus && !isJobSucceeded(jobStatus))) && (
              <WorkspaceJobProgress
                message={jobStatusError?.message ?? jobStatus?.message}
                progress={jobStatus?.progress}
                isFailed={isJobFailed(jobStatus)}
                startedAt={jobStatus?.startedAt}
                completedAt={jobStatus?.completedAt}
                isTerminal={isJobTerminal(jobStatus)}
                latestActivity={jobStatus?.latestActivity}
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
}
