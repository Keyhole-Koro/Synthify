-- name: GetOrCreateAccount :one
INSERT INTO accounts (
  account_id,
  name,
  plan,
  storage_quota_bytes,
  storage_used_bytes,
  max_file_size_bytes,
  max_uploads_per_5h,
  max_uploads_per_1week,
  stripe_customer_id,
  stripe_subscription_id,
  created_at,
  updated_at
)
VALUES ($1, $2, $3, $4, 0, $5, $6, $7, '', '', $8, $8)
ON CONFLICT (account_id) DO UPDATE SET account_id = EXCLUDED.account_id
RETURNING account_id, name, plan, storage_quota_bytes, storage_used_bytes, max_file_size_bytes, max_uploads_per_5h, max_uploads_per_1week, stripe_customer_id, stripe_subscription_id, created_at, updated_at;

-- name: GetAccount :one
SELECT account_id, name, plan, storage_quota_bytes, storage_used_bytes, max_file_size_bytes, max_uploads_per_5h, max_uploads_per_1week, stripe_customer_id, stripe_subscription_id, created_at, updated_at
FROM accounts
WHERE account_id = $1;

-- name: GetAccountByUser :one
SELECT a.account_id, a.name, a.plan, a.storage_quota_bytes, a.storage_used_bytes, a.max_file_size_bytes, a.max_uploads_per_5h, a.max_uploads_per_1week, a.stripe_customer_id, a.stripe_subscription_id, a.created_at, a.updated_at
FROM accounts a
JOIN account_users au ON au.account_id = a.account_id
WHERE au.user_id = $1
LIMIT 1;

-- name: IsAccountAccessible :one
SELECT EXISTS(
  SELECT 1 FROM account_users
  WHERE account_id = $1 AND user_id = $2
)::bool AS accessible;

-- name: CreateAccountUser :exec
INSERT INTO account_users (account_id, user_id, role, joined_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id, user_id) DO NOTHING;

-- name: ListWorkspacesByUser :many
SELECT w.workspace_id, w.account_id, w.name, w.created_at
FROM workspaces w
JOIN account_users au ON au.account_id = w.account_id
WHERE au.user_id = $1
ORDER BY w.created_at DESC;

-- name: GetWorkspace :one
SELECT workspace_id, account_id, name, created_at
FROM workspaces
WHERE workspace_id = $1;

-- name: IsWorkspaceAccessible :one
SELECT EXISTS(
  SELECT 1 FROM workspaces w
  JOIN account_users au ON au.account_id = w.account_id
  WHERE w.workspace_id = $1 AND au.user_id = $2
)::bool AS accessible;

-- name: CreateWorkspace :exec
INSERT INTO workspaces (workspace_id, account_id, name, created_at, updated_at)
VALUES ($1, $2, $3, $4, $4);

-- name: UpdateWorkspaceName :execrows
UPDATE workspaces
SET name = $2, updated_at = $3
WHERE workspace_id = $1;
