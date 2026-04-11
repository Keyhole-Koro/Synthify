-- name: ListNodesByDocument :many
SELECT node_id, document_id, label, level, category, entity_type, description, summary_html, created_by, created_at
FROM nodes
WHERE document_id = $1
ORDER BY level ASC, created_at ASC;

-- name: ListEdgesByDocument :many
SELECT edge_id, document_id, source_node_id, target_node_id, edge_type, description, created_at
FROM edges
WHERE document_id = $1
ORDER BY created_at ASC;

-- name: GetNode :one
SELECT node_id, document_id, label, level, category, entity_type, description, summary_html, created_by, created_at
FROM nodes
WHERE node_id = $1;

-- name: ListNodeEdges :many
SELECT edge_id, document_id, source_node_id, target_node_id, edge_type, description, created_at
FROM edges
WHERE source_node_id = $1 OR target_node_id = $1
ORDER BY created_at ASC;

-- name: CreateNode :exec
INSERT INTO nodes (node_id, document_id, label, level, category, entity_type, description, summary_html, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: CreateEdge :exec
INSERT INTO edges (edge_id, document_id, source_node_id, target_node_id, edge_type, description, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: InsertNodeView :exec
INSERT INTO node_views (workspace_id, user_id, node_id, document_id, viewed_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetWorkspaceMemberEmail :one
SELECT email
FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2;

-- name: ListViewedNodes :many
SELECT nv.node_id, nv.document_id, COALESCE(n.label, nv.node_id) AS label, MAX(nv.viewed_at)::timestamptz AS last_viewed_at, COUNT(*) AS view_count
FROM node_views nv
LEFT JOIN nodes n ON n.node_id = nv.node_id
WHERE nv.workspace_id = sqlc.arg(workspace_id)
  AND nv.user_id = sqlc.arg(user_id)
  AND (sqlc.arg(document_id_filter)::text = '' OR nv.document_id = sqlc.arg(document_id_filter)::text)
GROUP BY nv.node_id, nv.document_id, label
ORDER BY last_viewed_at DESC
LIMIT sqlc.arg(row_limit);

-- name: ListCreatedNodes :many
SELECT node_id, document_id, label, created_at
FROM nodes
WHERE created_by = sqlc.arg(created_by)
  AND (sqlc.arg(document_id_filter)::text = '' OR document_id = sqlc.arg(document_id_filter)::text)
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: UpsertApprovedAlias :exec
INSERT INTO node_aliases (workspace_id, canonical_node_id, alias_node_id, status, updated_at)
VALUES ($1, $2, $3, 'approved', $4)
ON CONFLICT (workspace_id, canonical_node_id, alias_node_id)
DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at;

-- name: UpsertRejectedAlias :exec
INSERT INTO node_aliases (workspace_id, canonical_node_id, alias_node_id, status, updated_at)
VALUES ($1, $2, $3, 'rejected', $4)
ON CONFLICT (workspace_id, canonical_node_id, alias_node_id)
DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at;
