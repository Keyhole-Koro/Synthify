-- name: ListWorkspaces :many
SELECT workspace_id, name, owner_id, plan, storage_used_bytes, storage_quota_bytes, max_file_size_bytes, max_uploads_per_day, created_at
FROM workspaces
ORDER BY created_at DESC;

-- name: GetWorkspace :one
SELECT workspace_id, name, owner_id, plan, storage_used_bytes, storage_quota_bytes, max_file_size_bytes, max_uploads_per_day, created_at
FROM workspaces
WHERE workspace_id = $1;

-- name: ListWorkspaceMembers :many
SELECT user_id, email, role, is_dev, invited_at, invited_by
FROM workspace_members
WHERE workspace_id = $1
ORDER BY invited_at ASC;

-- name: CreateWorkspace :exec
INSERT INTO workspaces (workspace_id, name, owner_id, plan, storage_used_bytes, storage_quota_bytes, max_file_size_bytes, max_uploads_per_day, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: CreateWorkspaceMember :exec
INSERT INTO workspace_members (workspace_id, user_id, email, role, is_dev, invited_at, invited_by)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateWorkspaceMemberRole :execrows
UPDATE workspace_members
SET role = $3, is_dev = $4
WHERE workspace_id = $1 AND user_id = $2;

-- name: GetWorkspaceMember :one
SELECT user_id, email, role, is_dev, invited_at, invited_by
FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2;

-- name: DeleteWorkspaceMember :execrows
DELETE FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2;

-- name: GetWorkspaceOwnerID :one
SELECT owner_id
FROM workspaces
WHERE workspace_id = $1;

-- name: PromoteWorkspaceOwner :execrows
UPDATE workspace_members
SET role = 'owner'
WHERE workspace_id = $1 AND user_id = $2;

-- name: DemoteWorkspaceOwner :exec
UPDATE workspace_members
SET role = 'editor'
WHERE workspace_id = $1 AND user_id = $2;

-- name: UpdateWorkspaceOwner :exec
UPDATE workspaces
SET owner_id = $2
WHERE workspace_id = $1;
