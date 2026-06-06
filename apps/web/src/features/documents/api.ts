import { createRPCClient } from '@/lib/connect';
import { DocumentService } from '@/gen/proto/synthify/app/v1/document_pb';
import type { Document } from '@/gen/proto/synthify/app/v1/document_pb';
import type { Job } from '@/gen/proto/synthify/app/v1/job_pb';
import { createAppError } from '@/lib/errors';

export type { Document } from '@/gen/proto/synthify/app/v1/document_pb';
export { DocumentLifecycleState } from '@/gen/proto/synthify/app/v1/document_pb';
export type { Job as ProcessingJob } from '@/gen/proto/synthify/app/v1/job_pb';

const client = createRPCClient(DocumentService);

export async function listDocuments(workspaceId: string): Promise<Document[]> {
  const res = await client.listDocuments({ workspaceId });
  return res.documents;
}

export async function getDocument(documentId: string): Promise<Document> {
  const res = await client.getDocument({ documentId });
  return res.document!;
}

export interface CreateDocumentResult {
  document: Document;
  uploadUrl: string;
  uploadMethod: string;
  uploadContentType: string;
}

export async function createDocument(
  workspaceId: string,
  filename: string,
  mimeType: string,
  fileSize: number,
): Promise<CreateDocumentResult> {
  const res = await client.createDocument({
    workspaceId,
    filename,
    mimeType,
    fileSize: BigInt(fileSize),
  });
  return {
    document: res.document!,
    uploadUrl: res.uploadUrl,
    uploadMethod: res.uploadMethod,
    uploadContentType: res.uploadContentType,
  };
}

export async function startProcessing(
  documentId: string,
  forceReprocess = false,
): Promise<{ documentId: string; job: Job }> {
  const res = await client.startProcessing({ documentId, forceReprocess });
  return { documentId: res.documentId, job: res.job! };
}

/**
 * Resolves a data-file-id image marker (embedded in generated HTML) to a
 * short-lived signed GET URL. Called at render time, so the URL is never
 * persisted into the saved HTML.
 */
export async function getImageURL(fileId: string): Promise<string> {
  const res = await client.getImageURL({ fileId });
  return res.url;
}

/** Uploads a file to a GCS signed URL with PUT or POST. */
export async function uploadFile(
  uploadUrl: string,
  file: File,
  method = 'PUT',
  contentType = file.type || 'application/octet-stream',
): Promise<void> {
  try {
    const res = await fetch(uploadUrl, {
      method,
      headers: { 'Content-Type': contentType },
      body: file,
    });
    if (!res.ok) {
      throw createAppError({
        kind: res.status === 413 ? 'validation' : res.status >= 500 ? 'server' : 'unknown',
        message: `アップロードに失敗しました (Status: ${res.status})`,
        retryable: res.status >= 500,
        code: res.status === 413 ? 'FILE_TOO_LARGE' : 'UPLOAD_FAILED',
        status: res.status,
      });
    }
  } catch (err) {
    if (err instanceof TypeError && err.message === 'Failed to fetch') {
      throw createAppError({
        kind: 'network',
        message: 'アップロード中にネットワークエラーが発生しました。',
        retryable: true,
        code: 'NETWORK_ERROR',
        cause: err,
      });
    }
    throw err;
  }
}
