'use client';

import React, { useRef, useState, useCallback, useEffect } from 'react';
import { useJobStatus } from '@/features/jobs/useJobStatus';
import { useWorkspaceJobStatuses } from '@/features/jobs/useWorkspaceJobStatuses';
import { WorkspaceHeader } from './components/WorkspaceHeader';
import { WorkspaceDropzone } from './components/WorkspaceDropzone';
import { WorkspaceJobProgress } from './components/WorkspaceJobProgress';
import { WorkspaceJobList } from './components/WorkspaceJobList';
import { WorkspaceBillingPanel } from './components/WorkspaceBillingPanel';
import { type Workspace } from '@/features/workspaces/api';

interface WorkspacePaperProps {
  workspace: Workspace;
  workspaceId: string;
  workspaceName: string;
  hasTree: boolean;
  childItems: { id: string }[];
  onUploadFile: (file: File) => Promise<{ jobId: string; documentId: string }>;
  onProcessingComplete?: (jobId: string) => Promise<void> | void;
}

export function WorkspacePaper({
  workspace,
  workspaceId,
  workspaceName,
  hasTree,
  childItems,
  onUploadFile,
  onProcessingComplete,
}: WorkspacePaperProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadMessage, setUploadMessage] = useState<string | null>(null);
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const completedJobRef = useRef<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [isHovered, setIsHovered] = useState(false);
  const [isPinned, setIsPinned] = useState(false);
  const { status: jobStatus, error: jobStatusError } = useJobStatus(workspaceId, activeJobId);
  const { jobs: workspaceJobs, error: workspaceJobsError } = useWorkspaceJobStatuses(workspaceId);

  const isTreeMissing = !hasTree;
  const isPopulated = hasTree && childItems.length > 0;
  const isExpanded = !isPopulated || isHovered || isPinned;
  const isRunning = !!activeJobId && jobStatus?.status === 'running';

  const handleUpload = async (file: File) => {
    setUploading(true);
    setUploadMessage(null);
    try {
      const job = await onUploadFile(file);
      setActiveJobId(job.jobId);
      completedJobRef.current = null;
      setUploadMessage('アップロードしました。解析を開始します。');
    } catch (err) {
      console.error('Upload failed:', err);
      setUploadMessage('アップロードに失敗しました。時間をおいて再試行してください。');
    } finally {
      setUploading(false);
    }
  };

  useEffect(() => {
    if (!jobStatus || !activeJobId) return;
    if (jobStatus.status !== 'succeeded') return;
    if (completedJobRef.current === activeJobId) return;
    completedJobRef.current = activeJobId;
    setUploadMessage('解析が完了しました。');
    void onProcessingComplete?.(activeJobId);
  }, [activeJobId, jobStatus, onProcessingComplete]);

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
      <input ref={fileInputRef} type="file" className="hidden" onChange={handleFileChange} />

      {/* Compact header (populated mode only) */}
      {isPopulated && (
        <WorkspaceHeader
          workspaceName={workspaceName}
          childItemsCount={childItems.length}
          isRunning={isRunning}
          jobProgress={jobStatus?.progress}
          isJustCompleted={uploadMessage === '解析が完了しました。'}
          isPinned={isPinned}
          onTogglePinned={() => setIsPinned((p) => !p)}
        />
      )}

      {/* Expanded content */}
      {isExpanded && (
        <div className={['flex flex-col gap-5 overflow-y-auto', isPopulated ? 'px-5 pb-5' : 'flex-1 p-5'].join(' ')}>
          {/* Header (empty mode) */}
          {!isPopulated && (
            <div>
              <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-indigo-400/80">Workspace</p>
              <h2 className="mt-0.5 text-lg font-semibold tracking-tight text-stone-800">{workspaceName}</h2>
            </div>
          )}

          {/* Upload zone / add button */}
          <div className={!isPopulated ? 'flex flex-1 flex-col justify-center' : ''}>
            <WorkspaceBillingPanel workspace={workspace} />

            <WorkspaceDropzone
              isTreeMissing={isTreeMissing}
              hasChildItems={childItems.length > 0}
              uploading={uploading}
              activeJobId={activeJobId}
              isDragging={isDragging}
              jobStatusMessage={jobStatusError ?? jobStatus?.message}
              jobStatusProgress={jobStatus?.progress}
              jobStatusFailed={jobStatus?.status === 'failed'}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
            />

            {uploadMessage && (
              <p className={[
                'mt-2.5 text-center text-[11px]',
                uploadMessage.includes('失敗') ? 'text-red-400' : 'text-indigo-500',
              ].join(' ')}>
                {uploadMessage}
              </p>
            )}

            {/* Progress: populated mode only (empty mode shows it inline inside the drop zone) */}
            {isPopulated && (jobStatus || jobStatusError) && (
              <WorkspaceJobProgress
                message={jobStatusError ?? jobStatus?.message}
                progress={jobStatus?.progress}
                isFailed={jobStatus?.status === 'failed'}
              />
            )}

            <WorkspaceJobList
              workspaceJobs={workspaceJobs}
              workspaceJobsError={workspaceJobsError}
            />
          </div>
        </div>
      )}
    </div>
  );
}
