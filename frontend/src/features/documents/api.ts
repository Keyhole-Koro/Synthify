import { callRPC } from '@/shared/lib/api';

export type DocumentStatus =
  | 'uploaded'
  | 'pending_normalization'
  | 'processing'
  | 'completed'
  | 'failed';

export interface Document {
  document_id: string;
  workspace_id: string;
  uploaded_by: string;
  filename: string;
  mime_type: string;
  file_size: number;
  status: DocumentStatus;
  extraction_depth?: string;
  node_count?: number;
  current_stage?: string;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export async function listDocuments(workspaceId: string): Promise<Document[]> {
  const res = await callRPC<{ workspace_id: string }, { documents: Document[] }>(
    'DocumentService',
    'ListDocuments',
    { workspace_id: workspaceId },
  );
  return res.documents ?? [];
}

export async function getDocument(documentId: string): Promise<Document> {
  const res = await callRPC<{ document_id: string }, { document: Document }>(
    'DocumentService',
    'GetDocument',
    { document_id: documentId },
  );
  return res.document;
}

export interface CreateDocumentResult {
  document: Document;
  upload_url: string;
  upload_method: string;
  upload_content_type: string;
}

export async function createDocument(
  workspaceId: string,
  filename: string,
  mimeType: string,
  fileSize: number,
): Promise<CreateDocumentResult> {
  return callRPC<
    { workspace_id: string; filename: string; mime_type: string; file_size: number },
    CreateDocumentResult
  >('DocumentService', 'CreateDocument', {
    workspace_id: workspaceId,
    filename,
    mime_type: mimeType,
    file_size: fileSize,
  });
}

export async function startProcessing(
  documentId: string,
  extractionDepth: 'full' | 'summary' = 'full',
  forceReprocess = false,
): Promise<{ document_id: string; status: string; job_id: string }> {
  return callRPC('DocumentService', 'StartProcessing', {
    document_id: documentId,
    extraction_depth: extractionDepth,
    force_reprocess: forceReprocess,
  });
}

/** ファイルを GCS 署名付き URL に PUT アップロードする。 */
export async function uploadFile(uploadUrl: string, file: File): Promise<void> {
  const res = await fetch(uploadUrl, {
    method: 'PUT',
    headers: { 'Content-Type': file.type },
    body: file,
  });
  if (!res.ok) {
    throw new Error(`Upload failed: ${res.status}`);
  }
}
