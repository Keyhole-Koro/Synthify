-- name: ListDocuments :many
SELECT document_id, workspace_id, uploaded_by, filename, mime_type, file_size, status, extraction_depth, node_count, current_stage, error_message, created_at, updated_at
FROM documents
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: GetDocument :one
SELECT document_id, workspace_id, uploaded_by, filename, mime_type, file_size, status, extraction_depth, node_count, current_stage, error_message, created_at, updated_at
FROM documents
WHERE document_id = $1;

-- name: CreateDocument :exec
INSERT INTO documents (document_id, workspace_id, uploaded_by, filename, mime_type, file_size, status, extraction_depth, node_count, current_stage, error_message, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: CountNodesByDocument :one
SELECT COUNT(*)
FROM nodes
WHERE document_id = $1;

-- name: CompleteDocumentProcessing :execrows
UPDATE documents
SET status = 'completed', extraction_depth = $2, current_stage = '', node_count = $3, updated_at = $4
WHERE document_id = $1;

-- name: UpdateDocumentTimestamp :exec
UPDATE documents
SET updated_at = $2
WHERE document_id = $1;
