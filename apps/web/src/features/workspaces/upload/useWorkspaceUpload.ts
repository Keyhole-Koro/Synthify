import { useCallback } from 'react';
import { createDocument, startProcessing, uploadFile } from '@/features/documents/api';
import { validateDocumentUploadFile } from '@/features/documents/uploadPolicy';
import { UserFacingError } from '@/lib/errors';

/**
 * Provides an upload pipeline callback that validates a candidate file, creates a document
 * metadata record, uploads the binary content to signed storage, and initiates AI processing.
 *
 * @returns An asynchronous handler function accepting `workspaceId` and `file`, returning `{ jobId, documentId }`.
 * The callback rejects with a UserFacingError for invalid files and propagates API, storage,
 * or processing failures. It performs validation before creating any server-side record.
 */
export function useWorkspaceUpload() {
  return useCallback(async (workspaceId: string, file: File) => {
    const validation = validateDocumentUploadFile(file);
    if (!validation.ok) {
      throw new UserFacingError(validation.error.message);
    }
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
