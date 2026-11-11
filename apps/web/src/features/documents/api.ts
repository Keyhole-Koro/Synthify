import { createRPCClient } from '@/lib/connect';
import { UserFacingError } from '@/lib/errors';
import { DocumentService } from '@/gen/proto/synthify/app/v1/document_pb';
import type { CreateDocumentResponse, Document } from '@/gen/proto/synthify/app/v1/document_pb';
import type { Job } from '@/gen/proto/synthify/app/v1/job_pb';

export type { Document } from '@/gen/proto/synthify/app/v1/document_pb';
export { DocumentLifecycleState } from '@/gen/proto/synthify/app/v1/document_pb';
export type { Job as ProcessingJob } from '@/gen/proto/synthify/app/v1/job_pb';

const client = createRPCClient(DocumentService);

/**
 * Lists all documents belonging to a workspace.
 *
 * @param workspaceId - Workspace identifier.
 * @returns Array of Document objects.
 */
export async function listDocuments(workspaceId: string): Promise<Document[]> {
  const res = await client.listDocuments({ workspaceId });
  return res.documents;
}

/**
 * Fetches a single document by its ID.
 *
 * @param documentId - Document identifier.
 * @returns The Document object.
 */
export async function getDocument(documentId: string): Promise<Document> {
  const res = await client.getDocument({ documentId });
  return res.document!;
}

/**
 * Result returned upon creating a document record: the proto
 * `CreateDocumentResponse` with `document` guaranteed to be present.
 */
export type CreateDocumentResult = Omit<CreateDocumentResponse, '$typeName' | '$unknown' | 'document'> & {
  document: Document;
};

/**
 * Registers a new document record and acquires a presigned URL for direct storage upload.
 *
 * @param workspaceId - Target workspace ID.
 * @param filename - Original file name.
 * @param mimeType - Document MIME type.
 * @param fileSize - File byte size.
 * @returns Created document and upload credentials.
 */
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

/**
 * Triggers background worker processing for an uploaded document.
 *
 * @param documentId - Target document ID.
 * @param forceReprocess - Whether to bypass cache and reprocess from scratch.
 * @returns Document ID and scheduled processing Job.
 */
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
 *
 * @param fileId - Storage file identifier taken from the HTML marker.
 * @returns Short-lived signed URL that can be used to load the image.
 * @throws Propagates the RPC error when the file cannot be resolved or the
 *   caller is not authorized to access it.
 */
export async function getImageURL(fileId: string): Promise<string> {
  const res = await client.getImageURL({ fileId });
  return res.url;
}

/**
 * Uploads a file directly to object storage using a signed URL.
 *
 * @param uploadUrl - Short-lived storage URL returned by `createDocument`.
 * @param file - Browser `File` whose bytes should be uploaded.
 * @param method - Signed-upload HTTP method, normally `PUT` or `POST`.
 * @param contentType - MIME type sent in the upload request; defaults to the
 *   file type or `application/octet-stream` when the file has no type.
 * @returns Promise fulfilled after the storage service accepts the upload.
 * @throws `UserFacingError` for an oversized file, a non-success response, or
 *   a network failure that prevents the request from completing.
 */
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
      if (res.status === 413) {
        throw new UserFacingError('ファイルサイズが上限を超えています。');
      }
      throw new UserFacingError('アップロードに失敗しました。');
    }
  } catch (err) {
    if (err instanceof TypeError && err.message === 'Failed to fetch') {
      throw new UserFacingError('アップロード中にネットワークエラーが発生しました。通信環境を確認してください。', {
        cause: err,
      });
    }
    throw err;
  }
}
