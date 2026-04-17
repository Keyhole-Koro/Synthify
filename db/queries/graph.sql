-- name: GetOrCreateGraph :one
INSERT INTO graphs (graph_id, workspace_id, name, created_at, updated_at)
VALUES ($1, $2, $3, $4, $4)
ON CONFLICT (workspace_id) DO UPDATE SET updated_at = EXCLUDED.updated_at
RETURNING graph_id, workspace_id, name, created_at, updated_at;

-- name: GetGraphByWorkspace :one
SELECT graph_id, workspace_id, name, created_at, updated_at
FROM graphs
WHERE workspace_id = $1;

-- name: ListNodesByGraph :many
SELECT node_id, graph_id, label, entity_type, description, summary_html, created_by, created_at
FROM nodes
WHERE graph_id = $1
ORDER BY created_at ASC;

-- name: ListEdgesByGraph :many
SELECT edge_id, graph_id, source_node_id, target_node_id, edge_type, description, created_at
FROM edges
WHERE graph_id = $1
ORDER BY created_at ASC;

-- name: GetNode :one
SELECT node_id, graph_id, label, entity_type, description, summary_html, created_by, created_at
FROM nodes
WHERE node_id = $1;

-- name: ListNodeEdges :many
SELECT edge_id, graph_id, source_node_id, target_node_id, edge_type, description, created_at
FROM edges
WHERE source_node_id = $1 OR target_node_id = $1
ORDER BY created_at ASC;

-- name: CreateNode :exec
INSERT INTO nodes (node_id, graph_id, label, category, entity_type, description, summary_html, created_by, created_at)
VALUES ($1, $2, $3, '', $4, $5, $6, $7, $8);

-- name: CreateEdge :exec
INSERT INTO edges (edge_id, graph_id, source_node_id, target_node_id, edge_type, description, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpsertNodeSource :exec
INSERT INTO node_sources (node_id, document_id, chunk_id, source_text, confidence)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (node_id, document_id, chunk_id)
DO UPDATE SET source_text = EXCLUDED.source_text, confidence = EXCLUDED.confidence;

-- name: UpsertEdgeSource :exec
INSERT INTO edge_sources (edge_id, document_id, chunk_id, source_text, confidence)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (edge_id, document_id, chunk_id)
DO UPDATE SET source_text = EXCLUDED.source_text, confidence = EXCLUDED.confidence;

-- name: ListNodeSources :many
SELECT node_id, document_id, chunk_id, source_text, COALESCE(confidence, 0) AS confidence
FROM node_sources
WHERE node_id = $1;

-- name: UpdateGraphTimestamp :exec
UPDATE graphs
SET updated_at = $2
WHERE graph_id = $1;

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

-- name: GetSubtreeNodes :many
WITH RECURSIVE subtree AS (
  SELECT node_id, graph_id, label, category, entity_type, description, summary_html, created_by, created_at, 0 AS rel_depth
  FROM nodes WHERE node_id = $1
  UNION ALL
  SELECT n.node_id, n.graph_id, n.label, n.category, n.entity_type, n.description, n.summary_html, n.created_by, n.created_at, s.rel_depth + 1
  FROM nodes n
  JOIN edges e ON e.target_node_id = n.node_id AND e.edge_type = 'hierarchical'
  JOIN subtree s ON s.node_id = e.source_node_id
  WHERE s.rel_depth < $2
)
SELECT node_id, graph_id, label, category, entity_type, description, summary_html, created_by, created_at,
  EXISTS(
    SELECT 1 FROM edges e2
    WHERE e2.source_node_id = subtree.node_id AND e2.edge_type = 'hierarchical'
  ) AS has_children
FROM subtree;

-- name: GetSubtreeEdges :many
WITH RECURSIVE subtree AS (
  SELECT node_id, 0 AS rel_depth FROM nodes WHERE node_id = $1
  UNION ALL
  SELECT n.node_id, s.rel_depth + 1
  FROM nodes n
  JOIN edges e ON e.target_node_id = n.node_id AND e.edge_type = 'hierarchical'
  JOIN subtree s ON s.node_id = e.source_node_id
  WHERE s.rel_depth < $2
)
SELECT e.edge_id, e.graph_id, e.source_node_id, e.target_node_id, e.edge_type, e.description, e.created_at
FROM edges e
JOIN subtree src ON src.node_id = e.source_node_id
JOIN subtree tgt ON tgt.node_id = e.target_node_id
WHERE e.edge_type = 'hierarchical';
