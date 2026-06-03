-- name: ListItemsByWorkspace :many
SELECT id, workspace_id, parent_id, title, level, description, content, override_css, created_by,
       governance_state, cross_document, created_at
FROM tree_items
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: ListRootItemsByWorkspace :many
-- Root nodes of the workspace tree: the items sitting directly under the
-- workspace (parent_id IS NULL). Replaces the old workspace_root lookup.
SELECT id, workspace_id, parent_id, title, level, description, content, override_css, created_by,
       governance_state, cross_document, created_at
FROM tree_items
WHERE workspace_id = $1 AND parent_id IS NULL
ORDER BY created_at ASC;

-- name: GetItem :one
SELECT id, workspace_id, parent_id, title, level, description, content, override_css, created_by,
       governance_state, cross_document, created_at
FROM tree_items
WHERE id = $1;

-- name: CreateItem :exec
-- Creates a node under an explicit parent. Pass NULL parent_id for a root
-- node (sitting directly under the workspace).
INSERT INTO tree_items (id, workspace_id, parent_id, title, level, description, content, override_css, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10);

-- name: CreateStructuredItem :exec
-- Creates a node with full governance/mutation metadata. parent_id NULL =
-- root node directly under the workspace.
INSERT INTO tree_items (
  id, workspace_id, parent_id, title, level, description, content, override_css,
  created_by, governance_state, last_mutation_job_id, cross_document, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13);

-- name: UpdateItemParent :exec
UPDATE tree_items
SET parent_id = $2, updated_at = $3
WHERE id = $1;

-- name: GetItemSummaryUpdateContext :one
SELECT workspace_id, content, governance_state
FROM tree_items
WHERE id = $1;

-- name: UpdateItemSummaryAndMutation :execrows
UPDATE tree_items
SET content = $2, last_mutation_job_id = $3, updated_at = $4
WHERE id = $1;

-- name: UpdateItemForMerge :execrows
-- Merge a document's knowledge into an existing node: the worker rewrites the
-- node's title/description/content and marks it cross_document. item_sources
-- gains the new document's provenance via UpsertItemSource.
UPDATE tree_items
SET title = $2, description = $3, content = $4, cross_document = TRUE,
    last_mutation_job_id = $5, updated_at = $6
WHERE id = $1;

-- name: UpsertItemSource :exec
INSERT INTO item_sources (item_id, document_id, file_id, chunk_id, source_text, confidence)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (item_id, document_id, chunk_id)
DO UPDATE SET file_id = EXCLUDED.file_id, source_text = EXCLUDED.source_text, confidence = EXCLUDED.confidence;

-- name: ListItemSources :many
SELECT s.item_id, s.document_id, s.file_id, f.path AS sub_path, s.chunk_id, s.source_text, COALESCE(s.confidence, 0) AS confidence
FROM item_sources s
LEFT JOIN document_files f ON f.file_id = s.file_id
WHERE s.item_id = $1;

-- name: UpdateTreeTimestamp :exec
UPDATE tree_items
SET updated_at = $2
WHERE workspace_id = (SELECT workspace_id FROM tree_items WHERE tree_items.id = $1 LIMIT 1);

-- name: UpsertApprovedAlias :exec
INSERT INTO item_aliases (workspace_id, canonical_item_id, alias_item_id, status, updated_at)
VALUES ($1, $2, $3, 'approved', $4)
ON CONFLICT (workspace_id, canonical_item_id, alias_item_id)
DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at;

-- name: UpsertRejectedAlias :exec
INSERT INTO item_aliases (workspace_id, canonical_item_id, alias_item_id, status, updated_at)
VALUES ($1, $2, $3, 'rejected', $4)
ON CONFLICT (workspace_id, canonical_item_id, alias_item_id)
DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at;

-- name: CountJobMutationsByTarget :one
SELECT COUNT(*)
FROM job_mutation_logs
WHERE job_id = $1 AND target_type = $2;

-- name: InsertJobMutationLog :exec
INSERT INTO job_mutation_logs (
  mutation_id, job_id, plan_id, capability_id, workspace_id, target_type, target_id, mutation_type,
  risk_tier, before_json, after_json, provenance_json, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: ListChildItems :many
SELECT id, workspace_id, parent_id, title, level, description, content, override_css, created_by,
  governance_state, cross_document, created_at,
  EXISTS(SELECT 1 FROM tree_items child WHERE child.parent_id = tree_items.id) AS has_children
FROM tree_items
WHERE tree_items.parent_id = $1;
