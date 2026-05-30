'use client';

import React, { useRef, useState } from 'react';
import { WorkspaceDropzone } from '@/features/workspaces/components/WorkspaceDropzone';
import { log } from '@/lib/observability/log';

interface RootUploadPaperProps {
  disabled: boolean;
  onUpload: (file: File) => Promise<void>;
}

export function RootUploadPaper({
  disabled,
  onUpload,
}: RootUploadPaperProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadMessage, setUploadMessage] = useState<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);

  async function handleUpload(file: File) {
    if (disabled || uploading) return;
    setUploading(true);
    setUploadMessage(null);
    try {
      await onUpload(file);
    } catch (err) {
      log.error('Root upload failed', { source: 'root_upload_paper' }, err);
      setUploadMessage('アップロードに失敗しました。時間をおいて再試行してください。');
    } finally {
      setUploading(false);
    }
  }

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    await handleUpload(file);
  }

  return (
    <div className="flex h-full flex-col justify-center gap-3 p-5">
      <input
        ref={fileInputRef}
        type="file"
        className="sr-only"
        onChange={handleFileChange}
        onClick={(e) => {
          (e.target as HTMLInputElement).value = '';
        }}
      />
      <WorkspaceDropzone
        isTreeMissing
        hasChildItems={false}
        uploading={uploading}
        activeJobId={null}
        isDragging={isDragging}
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
