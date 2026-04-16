-- name: ListDocuments :many
SELECT document_id, workspace_id, uploaded_by, filename, mime_type, file_size, created_at
FROM documents
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: GetDocument :one
SELECT document_id, workspace_id, uploaded_by, filename, mime_type, file_size, created_at
FROM documents
WHERE document_id = $1;

-- name: CreateDocument :exec
INSERT INTO documents (document_id, workspace_id, uploaded_by, filename, mime_type, file_size, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetLatestProcessingJob :one
SELECT job_id, document_id, COALESCE(graph_id, '') AS graph_id, job_type, status, current_stage, error_message, params_json, created_at, updated_at
FROM document_processing_jobs
WHERE document_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateProcessingJob :exec
INSERT INTO document_processing_jobs (job_id, document_id, graph_id, job_type, status, current_stage, error_message, params_json, created_at, updated_at)
VALUES ($1, $2, NULLIF($3, ''), $4, $5, '', '', '{}', $6, $6);

-- name: CompleteProcessingJob :execrows
UPDATE document_processing_jobs
SET status = 'completed', current_stage = '', updated_at = $2
WHERE job_id = $1;

-- name: FailProcessingJob :execrows
UPDATE document_processing_jobs
SET status = 'failed', error_message = $2, updated_at = $3
WHERE job_id = $1;

-- name: UpdateProcessingJobStage :exec
UPDATE document_processing_jobs
SET current_stage = $2, updated_at = $3
WHERE job_id = $1;
