import { callRPC } from '@/lib/rpc';

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

interface ConnectDocument {
  documentId: string;
  workspaceId: string;
  uploadedBy?: string;
  filename: string;
  mimeType: string;
  fileSize: number;
  status: string;
  extractionDepth?: string;
  nodeCount?: number;
  currentStage?: string;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
}

function mapDocumentStatus(status: string): DocumentStatus {
  switch (status) {
    case 'DOCUMENT_LIFECYCLE_STATE_PENDING_NORMALIZATION':
      return 'pending_normalization';
    case 'DOCUMENT_LIFECYCLE_STATE_PROCESSING':
      return 'processing';
    case 'DOCUMENT_LIFECYCLE_STATE_COMPLETED':
      return 'completed';
    case 'DOCUMENT_LIFECYCLE_STATE_FAILED':
      return 'failed';
    default:
      return 'uploaded';
  }
}

function mapExtractionDepth(depth?: string): string | undefined {
  switch (depth) {
    case 'EXTRACTION_DEPTH_FULL':
      return 'full';
    case 'EXTRACTION_DEPTH_SUMMARY':
      return 'summary';
    default:
      return undefined;
  }
}

function mapDocument(document: ConnectDocument): Document {
  return {
    document_id: document.documentId,
    workspace_id: document.workspaceId,
    uploaded_by: document.uploadedBy ?? '',
    filename: document.filename,
    mime_type: document.mimeType,
    file_size: document.fileSize,
    status: mapDocumentStatus(document.status),
    extraction_depth: mapExtractionDepth(document.extractionDepth),
    node_count: document.nodeCount,
    current_stage: document.currentStage,
    error_message: document.errorMessage,
    created_at: document.createdAt,
    updated_at: document.updatedAt,
  };
}

export async function listDocuments(workspaceId: string): Promise<Document[]> {
  const res = await callRPC<{ workspaceId: string }, { documents: ConnectDocument[] }>(
    'DocumentService',
    'ListDocuments',
    { workspaceId },
  );
  return (res.documents ?? []).map(mapDocument);
}

export async function getDocument(documentId: string): Promise<Document> {
  const res = await callRPC<{ documentId: string }, { document: ConnectDocument }>(
    'DocumentService',
    'GetDocument',
    { documentId },
  );
  return mapDocument(res.document);
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
  const res = await callRPC<
    { workspaceId: string; filename: string; mimeType: string; fileSize: number },
    { document: ConnectDocument; uploadUrl: string; uploadMethod: string; uploadContentType: string }
  >('DocumentService', 'CreateDocument', {
    workspaceId,
    filename,
    mimeType,
    fileSize,
  });
  return {
    document: mapDocument(res.document),
    upload_url: res.uploadUrl,
    upload_method: res.uploadMethod,
    upload_content_type: res.uploadContentType,
  };
}

export async function startProcessing(
  documentId: string,
  extractionDepth: 'full' | 'summary' = 'full',
  forceReprocess = false,
): Promise<{ document_id: string; status: string; job_id: string }> {
  const res = await callRPC<
    { documentId: string; extractionDepth: string; forceReprocess: boolean },
    { documentId: string; job: { jobId: string; status: string } }
  >('DocumentService', 'StartProcessing', {
    documentId,
    extractionDepth: extractionDepth === 'summary' ? 'EXTRACTION_DEPTH_SUMMARY' : 'EXTRACTION_DEPTH_FULL',
    forceReprocess,
  });
  return {
    document_id: res.documentId,
    status: res.job?.status ?? 'JOB_LIFECYCLE_STATE_QUEUED',
    job_id: res.job?.jobId ?? '',
  };
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
