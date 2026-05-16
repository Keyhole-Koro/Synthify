import { createRPCClient } from '@/lib/connect';
import { DocumentService } from '@synthify/proto-ts/gen/synthify/tree/v1/document_pb';
import type { Document } from '@synthify/proto-ts/gen/synthify/tree/v1/document_pb';
import type { Job } from '@synthify/proto-ts/gen/synthify/tree/v1/job_pb';

export type { Document } from '@synthify/proto-ts/gen/synthify/tree/v1/document_pb';
export { DocumentLifecycleState } from '@synthify/proto-ts/gen/synthify/tree/v1/document_pb';
export type { Job as ProcessingJob } from '@synthify/proto-ts/gen/synthify/tree/v1/job_pb';

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

/** Uploads a file to a GCS signed URL with PUT or POST. */
export async function uploadFile(
  uploadUrl: string,
  file: File,
  method = 'PUT',
): Promise<void> {
  const res = await fetch(uploadUrl, {
    method,
    headers: { 'Content-Type': file.type },
    body: file,
  });
  if (!res.ok) {
    throw new Error(`Upload failed: ${res.status}`);
  }
}
