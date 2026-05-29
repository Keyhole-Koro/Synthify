'use client';

import { useCallback } from 'react';
import { createDocument, startProcessing, uploadFile } from '@/features/documents/api';

// useWorkspaceUpload returns the workspace upload pipeline: create document
// row -> upload bytes -> start processing job. The caller decides what to
// do with the resulting jobId / documentId (typically injecting it into the
// workspace runtime state so the progress UI lights up).
export function useWorkspaceUpload() {
  return useCallback(async (workspaceId: string, file: File) => {
    const created = await createDocument(
      workspaceId,
      file.name,
      file.type || 'application/octet-stream',
      file.size,
    );
    await uploadFile(created.uploadUrl, file, created.uploadMethod, created.uploadContentType);
    const processing = await startProcessing(created.document.documentId);
    return {
      jobId: processing.job.jobId,
      documentId: created.document.documentId,
    };
  }, []);
}
