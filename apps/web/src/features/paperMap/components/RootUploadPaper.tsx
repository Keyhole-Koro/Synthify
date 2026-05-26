'use client';

import React, { useEffect, useRef, useState } from 'react';
import { WorkspaceDropzone } from '@/features/workspaces/components/WorkspaceDropzone';
import { useJobStatus } from '@/features/jobs/useJobStatus';

interface RootUploadResult {
  workspaceId: string;
  workspaceName: string;
  jobId: string;
  documentId: string;
}

interface RootUploadPaperProps {
  disabled: boolean;
  onUpload: (file: File) => Promise<RootUploadResult>;
  onProcessingComplete: (workspaceId: string) => Promise<void> | void;
  onSuggestedWorkspaceName: (workspaceId: string, name: string) => Promise<void> | void;
}

export function RootUploadPaper({
  disabled,
  onUpload,
  onProcessingComplete,
  onSuggestedWorkspaceName,
}: RootUploadPaperProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const completedJobRef = useRef<string | null>(null);
  const suggestedNameRef = useRef<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadMessage, setUploadMessage] = useState<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [activeJob, setActiveJob] = useState<RootUploadResult | null>(null);
  const { status: jobStatus, error: jobStatusError } = useJobStatus(activeJob?.workspaceId ?? '', activeJob?.jobId ?? null);

  async function handleUpload(file: File) {
    if (disabled || uploading) return;
    setUploading(true);
    setUploadMessage(null);
    try {
      const result = await onUpload(file);
      setActiveJob(result);
      completedJobRef.current = null;
      suggestedNameRef.current = null;
      setUploadMessage(`${result.workspaceName} で解析を開始しました。`);
    } catch (err) {
      console.error('Root upload failed:', err);
      setUploadMessage('アップロードに失敗しました。時間をおいて再試行してください。');
    } finally {
      setUploading(false);
    }
  }

  useEffect(() => {
    if (!activeJob || !jobStatus?.suggestedWorkspaceName) return;
    if (suggestedNameRef.current === jobStatus.suggestedWorkspaceName) return;
    suggestedNameRef.current = jobStatus.suggestedWorkspaceName;
    void onSuggestedWorkspaceName(activeJob.workspaceId, jobStatus.suggestedWorkspaceName);
  }, [activeJob, jobStatus?.suggestedWorkspaceName, onSuggestedWorkspaceName]);

  useEffect(() => {
    if (!activeJob || !jobStatus || jobStatus.status !== 'succeeded') return;
    if (completedJobRef.current === activeJob.jobId) return;
    completedJobRef.current = activeJob.jobId;
    setUploadMessage('解析が完了しました。');
    void onProcessingComplete(activeJob.workspaceId);
  }, [activeJob, jobStatus, onProcessingComplete]);

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    await handleUpload(file);
  }

  return (
    <div className="flex h-full flex-col justify-center gap-3 p-5">
      <input ref={fileInputRef} type="file" className="hidden" onChange={handleFileChange} />
      <WorkspaceDropzone
        isTreeMissing
        hasChildItems={false}
        uploading={uploading}
        activeJobId={activeJob?.jobId ?? null}
        isDragging={isDragging}
        jobStatusMessage={jobStatusError ?? jobStatus?.message}
        jobStatusProgress={jobStatus?.progress}
        jobStatusFailed={jobStatus?.status === 'failed'}
        onDragOver={(e) => {
          e.preventDefault();
          if (!disabled) setIsDragging(true);
        }}
        onDragLeave={(e) => {
          if (!e.currentTarget.contains(e.relatedTarget as Node)) {
            setIsDragging(false);
          }
        }}
        onDrop={(e) => {
          e.preventDefault();
          setIsDragging(false);
          const file = e.dataTransfer.files[0];
          if (file) void handleUpload(file);
        }}
        onClick={() => {
          if (!disabled) fileInputRef.current?.click();
        }}
      />
      {uploadMessage && (
        <p
          className={[
            'text-center text-[11px]',
            uploadMessage.includes('失敗') ? 'text-red-400' : 'text-indigo-500',
          ].join(' ')}
        >
          {uploadMessage}
        </p>
      )}
    </div>
  );
}
