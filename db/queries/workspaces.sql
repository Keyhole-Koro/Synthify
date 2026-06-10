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
-- account 経由 (所有) と workspace_members 経由 (被共有) の両方を返す。
-- 両方に該当する workspace は UNION で 1 行に重複排除される。
SELECT workspace_id, account_id, name, created_at
FROM (
  SELECT w.workspace_id, w.account_id, w.name, w.created_at
  FROM workspaces w
  JOIN account_users au ON au.account_id = w.account_id
  WHERE au.user_id = $1 AND w.deleted_at IS NULL
  UNION
  SELECT w.workspace_id, w.account_id, w.name, w.created_at
  FROM workspaces w
  JOIN workspace_members wm ON wm.workspace_id = w.workspace_id
  WHERE wm.user_id = $1 AND w.deleted_at IS NULL
) AS accessible
ORDER BY created_at DESC;

-- name: GetWorkspace :one
SELECT workspace_id, account_id, name, created_at
FROM workspaces
WHERE workspace_id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteWorkspace :execrows
UPDATE workspaces
SET deleted_at = $2, updated_at = $2
WHERE workspace_id = $1;

-- name: IsWorkspaceAccessible :one
-- account 経由 (フラットモデル) OR workspace_members 経由のどちらかでアクセス可。
SELECT EXISTS(
  SELECT 1 FROM workspaces w
  JOIN account_users au ON au.account_id = w.account_id
  WHERE w.workspace_id = $1 AND au.user_id = $2 AND w.deleted_at IS NULL
  UNION ALL
  SELECT 1 FROM workspaces w
  JOIN workspace_members wm ON wm.workspace_id = w.workspace_id
  WHERE w.workspace_id = $1 AND wm.user_id = $2 AND w.deleted_at IS NULL
)::bool AS accessible;

-- name: GetWorkspaceRole :one
-- アクセス可能なら role を返す。account 経由は owner 相当。
-- 両方該当する場合は account(owner) を優先する。
SELECT CASE
  WHEN EXISTS(
    SELECT 1 FROM workspaces w
    JOIN account_users au ON au.account_id = w.account_id
    WHERE w.workspace_id = $1 AND au.user_id = $2 AND w.deleted_at IS NULL
  ) THEN 'owner'
  ELSE COALESCE(
    (SELECT wm.role FROM workspace_members wm
     JOIN workspaces w ON w.workspace_id = wm.workspace_id
     WHERE wm.workspace_id = $1 AND wm.user_id = $2 AND w.deleted_at IS NULL),
    ''
  )
END::text AS role;

-- --- Workspace members ---------------------------------------------------

-- name: UpsertWorkspaceMember :exec
INSERT INTO workspace_members (workspace_id, user_id, role, invited_by, invited_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, user_id) DO UPDATE SET role = EXCLUDED.role;

-- name: ListWorkspaceMembers :many
SELECT wm.workspace_id, wm.user_id, COALESCE(u.email, '')::text AS email,
       wm.role, wm.invited_by, wm.invited_at
FROM workspace_members wm
LEFT JOIN users u ON u.user_id = wm.user_id
WHERE wm.workspace_id = $1
ORDER BY wm.invited_at ASC;

-- name: RemoveWorkspaceMember :execrows
DELETE FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2;

-- --- Workspace share links -----------------------------------------------

-- name: CreateShareLink :exec
INSERT INTO workspace_share_links (token, workspace_id, role, created_by, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListShareLinks :many
SELECT token, workspace_id, role, created_by, expires_at, revoked_at, created_at
FROM workspace_share_links
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: RevokeShareLink :execrows
UPDATE workspace_share_links
SET revoked_at = $3
WHERE workspace_id = $1 AND token = $2 AND revoked_at IS NULL;

-- name: ResolveShareLink :one
-- 有効な (未失効・未期限切れ) リンクのみ workspace と role を返す。
SELECT sl.token, sl.workspace_id, sl.role, sl.created_by, sl.expires_at, sl.revoked_at, sl.created_at
FROM workspace_share_links sl
JOIN workspaces w ON w.workspace_id = sl.workspace_id
WHERE sl.token = $1
  AND sl.revoked_at IS NULL
  AND (sl.expires_at IS NULL OR sl.expires_at > $2)
  AND w.deleted_at IS NULL;

-- name: CreateWorkspace :exec
INSERT INTO workspaces (workspace_id, account_id, name, created_at, updated_at)
VALUES ($1, $2, $3, $4, $4);

-- name: UpdateWorkspaceName :execrows
UPDATE workspaces
SET name = $2, updated_at = $3
WHERE workspace_id = $1 AND deleted_at IS NULL;
