-- name: GetTreeRoot :one
SELECT id, workspace_id, parent_id, label, level, description, summary_html, created_by, created_at
FROM tree_items
WHERE workspace_id = $1 AND parent_id IS NULL
LIMIT 1;

-- name: ListItemsByWorkspace :many
SELECT id, workspace_id, parent_id, label, level, description, summary_html, created_by,
       COALESCE(governance_state, 'system_generated') AS governance_state, created_at
FROM tree_items
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: GetItem :one
SELECT id, workspace_id, parent_id, label, level, description, summary_html, created_by,
       COALESCE(governance_state, 'system_generated') AS governance_state, created_at
FROM tree_items
WHERE id = $1;

-- name: CreateItem :exec
INSERT INTO tree_items (id, workspace_id, parent_id, label, level, description, summary_html, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9);

-- name: CreateStructuredItem :exec
INSERT INTO tree_items (
  id, workspace_id, parent_id, label, level, description, summary_html,
  created_by, governance_state, last_mutation_job_id, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11);

-- name: UpdateItemParent :exec
UPDATE tree_items
SET parent_id = $2, updated_at = $3
WHERE id = $1;

-- name: GetItemSummaryUpdateContext :one
SELECT workspace_id, COALESCE(summary_html, '') AS summary_html, COALESCE(governance_state, 'system_generated') AS governance_state
FROM tree_items
WHERE id = $1;

-- name: UpdateItemSummaryAndMutation :execrows
UPDATE tree_items
SET summary_html = $2, last_mutation_job_id = $3, updated_at = $4
WHERE id = $1;

-- name: UpsertItemSource :exec
INSERT INTO item_sources (item_id, document_id, chunk_id, source_text, confidence)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (item_id, document_id, chunk_id)
DO UPDATE SET source_text = EXCLUDED.source_text, confidence = EXCLUDED.confidence;

-- name: ListItemSources :many
SELECT item_id, document_id, chunk_id, source_text, COALESCE(confidence, 0) AS confidence
FROM item_sources
WHERE item_id = $1;

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
SELECT id, workspace_id, parent_id, label, level, description, summary_html, created_by,
  COALESCE(governance_state, 'system_generated') AS governance_state, created_at,
  EXISTS(SELECT 1 FROM tree_items child WHERE child.parent_id = tree_items.id) AS has_children
FROM tree_items
WHERE tree_items.parent_id = $1;
