import { callRPC } from '@/lib/rpc';

export interface Document {
  document_id: string;
  workspace_id: string;
  uploaded_by: string;
  filename: string;
  mime_type: string;
  file_size: number;
  created_at: string;
}

export type JobStatus = 'queued' | 'running' | 'completed' | 'failed';

export interface DocumentProcessingJob {
  job_id: string;
  document_id: string;
  graph_id: string;
  job_type: string;
  status: JobStatus;
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
  createdAt: string;
  updatedAt: string;
}

interface ConnectJob {
  jobId: string;
  documentId: string;
  graphId?: string;
  jobType: string;
  status: string;
  currentStage?: string;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
}

function mapDocument(document: ConnectDocument): Document {
  return {
    document_id: document.documentId,
    workspace_id: document.workspaceId,
    uploaded_by: document.uploadedBy ?? '',
    filename: document.filename,
    mime_type: document.mimeType,
    file_size: document.fileSize,
    created_at: document.createdAt,
  };
}

function mapJobStatus(status: string): JobStatus {
  switch (status) {
    case 'JOB_LIFECYCLE_STATE_RUNNING':
      return 'running';
    case 'JOB_LIFECYCLE_STATE_COMPLETED':
      return 'completed';
    case 'JOB_LIFECYCLE_STATE_FAILED':
      return 'failed';
    default:
      return 'queued';
  }
}

function mapJob(job: ConnectJob): DocumentProcessingJob {
  return {
    job_id: job.jobId,
    document_id: job.documentId,
    graph_id: job.graphId ?? '',
    job_type: job.jobType,
    status: mapJobStatus(job.status),
    current_stage: job.currentStage,
    error_message: job.errorMessage,
    created_at: job.createdAt,
    updated_at: job.updatedAt,
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

export async function getLatestProcessingJob(
  documentId: string,
): Promise<DocumentProcessingJob | null> {
  const res = await callRPC<
    { documentId: string },
    { job?: ConnectJob }
  >('DocumentService', 'GetLatestProcessingJob', { documentId });
  return res.job ? mapJob(res.job) : null;
}

export async function startProcessing(
  documentId: string,
  forceReprocess = false,
): Promise<{ document_id: string; job: DocumentProcessingJob }> {
  const res = await callRPC<
    { documentId: string; forceReprocess: boolean },
    { documentId: string; job: ConnectJob }
  >('DocumentService', 'StartProcessing', {
    documentId,
    forceReprocess,
  });
  return {
    document_id: res.documentId,
    job: mapJob(res.job),
  };
}

/** Uploads a file to a GCS signed URL with PUT. */
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
