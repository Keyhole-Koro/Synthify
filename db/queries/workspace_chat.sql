-- name: CreateWorkspaceChatConversation :one
INSERT INTO workspace_chat_conversations (conversation_id, workspace_id, created_by, title, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
RETURNING conversation_id, workspace_id, created_by, title, created_at, updated_at;

-- name: GetWorkspaceChatConversation :one
SELECT conversation_id, workspace_id, created_by, title, created_at, updated_at
FROM workspace_chat_conversations
WHERE conversation_id = $1;

-- name: ListWorkspaceChatConversations :many
SELECT conversation_id, workspace_id, created_by, title, created_at, updated_at
FROM workspace_chat_conversations
WHERE workspace_id = $1
ORDER BY updated_at DESC
LIMIT $2;

-- name: TouchWorkspaceChatConversation :exec
UPDATE workspace_chat_conversations
SET updated_at = $2
WHERE conversation_id = $1;

-- name: CreateWorkspaceChatMessage :one
INSERT INTO workspace_chat_messages (
  message_id, conversation_id, role, content, model_selection, status, error_code, retrieval_snapshot_json, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING message_id, conversation_id, role, content, model_selection, status, error_code, created_at;

-- name: ListWorkspaceChatMessages :many
SELECT message_id, conversation_id, role, content, model_selection, status, error_code, created_at
FROM workspace_chat_messages
WHERE conversation_id = $1
ORDER BY created_at, message_id
LIMIT $2;

-- ListRecentWorkspaceChatMessages returns the newest N messages for prompt history.
-- Caller reverses the slice to get chronological order.
-- name: ListRecentWorkspaceChatMessages :many
SELECT message_id, conversation_id, role, content, model_selection, status, error_code, created_at
FROM workspace_chat_messages
WHERE conversation_id = $1 AND status = 'complete'
ORDER BY created_at DESC, message_id DESC
LIMIT $2;

-- name: CreateWorkspaceChatMessageSource :exec
INSERT INTO workspace_chat_message_sources (message_id, ordinal, document_id, chunk_id, item_id, label)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListWorkspaceChatMessageSources :many
SELECT s.message_id, s.ordinal, s.document_id, s.chunk_id, s.item_id, s.label
FROM workspace_chat_message_sources s
INNER JOIN workspace_chat_messages m ON m.message_id = s.message_id
WHERE m.conversation_id = $1
ORDER BY s.message_id, s.ordinal;

-- name: CountWorkspaceChatMessagesByRole :one
SELECT COUNT(*)::bigint AS message_count
FROM workspace_chat_messages
WHERE conversation_id = $1 AND role = $2;

-- SearchWorkspaceChatSourceChunks retrieves candidate chunks for grounding.
-- It is scoped by workspace_id, never by chunk id alone, and excludes documents
-- with no succeeded processing job (so in-flight uploads cannot be cited).
-- The 'succeeded' literal must match jobstatus.LifecycleStateToDB.
--
-- NOTE: this requires a pgvector-capable database. The local dev stack runs
-- CockroachDB v24.2, which has no vector_cosine_distance, so this query fails
-- there — same dormant state as SearchWorkspaceDocumentChunksByVector, which is
-- generated but not yet called from Go. Retrieval therefore uses the lexical
-- query below as its working path and only reaches for this one where embeddings
-- and pgvector are both actually available.
-- name: SearchWorkspaceChatSourceChunks :many
SELECT c.chunk_id, c.document_id, d.filename, f.path AS sub_path, c.heading, c.text, c.source_page,
       1 - vector_cosine_distance(c.embedding, sqlc.arg(query_embedding)) AS similarity
FROM document_chunks c
INNER JOIN documents d ON d.document_id = c.document_id
LEFT JOIN document_files f ON f.file_id = c.file_id
WHERE d.workspace_id = sqlc.arg(workspace_id)
  AND c.embedding IS NOT NULL
  AND EXISTS (
    SELECT 1 FROM document_processing_jobs j
    WHERE j.document_id = d.document_id AND j.status = 'succeeded'
  )
ORDER BY vector_cosine_distance(c.embedding, sqlc.arg(query_embedding))
LIMIT sqlc.arg(result_limit);

-- ListWorkspaceChatSourceChunksLexical is the deterministic fallback used when the
-- query has no embedding (embedding provider down, or an empty-vector query).
-- name: ListWorkspaceChatSourceChunksLexical :many
SELECT c.chunk_id, c.document_id, d.filename, f.path AS sub_path, c.heading, c.text, c.source_page
FROM document_chunks c
INNER JOIN documents d ON d.document_id = c.document_id
LEFT JOIN document_files f ON f.file_id = c.file_id
WHERE d.workspace_id = sqlc.arg(workspace_id)
  AND EXISTS (
    SELECT 1 FROM document_processing_jobs j
    WHERE j.document_id = d.document_id AND j.status = 'succeeded'
  )
  AND (
    sqlc.arg(query_text)::text = ''
    OR c.text ILIKE '%' || sqlc.arg(query_text)::text || '%'
    OR c.heading ILIKE '%' || sqlc.arg(query_text)::text || '%'
  )
ORDER BY c.document_id, c.chunk_id
LIMIT sqlc.arg(result_limit);

-- CountWorkspaceChatSourceDocuments reports whether the workspace has anything
-- answerable at all, so the API can return chat_source_unavailable before it
-- spends a model call.
-- name: CountWorkspaceChatSourceDocuments :one
SELECT COUNT(DISTINCT d.document_id)::bigint AS document_count
FROM documents d
WHERE d.workspace_id = $1
  AND EXISTS (
    SELECT 1 FROM document_processing_jobs j
    WHERE j.document_id = d.document_id AND j.status = 'succeeded'
  );
