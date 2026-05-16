-- name: MaxDynamicToolVersion :one
SELECT COALESCE(MAX(version), 0)::int AS max_version
FROM dynamic_tools
WHERE scope = $1 AND origin_workspace_id = $2 AND name = $3;

-- name: InsertDynamicTool :exec
INSERT INTO dynamic_tools (
  tool_id, name, version, scope, origin_workspace_id, origin_job_id,
  description, language, code, io_schema,
  declared_tier, floor_tier, risk_tier, status,
  input_sample, created_at
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8, $9, $10,
  $11, $12, $13, $14,
  $15, $16
);

-- name: ListDynamicToolCandidates :many
SELECT tool_id, name, version, scope, origin_workspace_id, origin_job_id,
       description, language, code, io_schema,
       declared_tier, floor_tier, risk_tier, status,
       input_sample, runtime_version, use_count, created_at, promoted_at, reviewed_by
FROM dynamic_tools
WHERE status = 'candidate'
ORDER BY created_at ASC
LIMIT $1;

-- name: ResolveActiveDynamicTools :many
SELECT tool_id, name, version, scope, origin_workspace_id, origin_job_id,
       description, language, code, io_schema,
       declared_tier, floor_tier, risk_tier, status,
       input_sample, runtime_version, use_count, created_at, promoted_at, reviewed_by
FROM dynamic_tools
WHERE status = 'active' AND (scope = 'global' OR origin_workspace_id = $1)
ORDER BY created_at ASC;

-- name: PromoteDynamicToolCandidate :execrows
UPDATE dynamic_tools
SET status = $2, promoted_at = $3
WHERE tool_id = $1;

-- name: PromoteDynamicToolToGlobal :execrows
UPDATE dynamic_tools
SET scope = 'global', reviewed_by = $2
WHERE tool_id = $1;

-- name: SetDynamicToolStatus :execrows
UPDATE dynamic_tools
SET status = $2
WHERE tool_id = $1;

-- name: IncrementDynamicToolUseCount :execrows
UPDATE dynamic_tools
SET use_count = use_count + 1
WHERE tool_id = $1;
