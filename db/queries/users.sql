-- name: GetUser :one
SELECT user_id, email, display_name, created_at, last_login_at, updated_at
FROM users
WHERE user_id = $1;

-- name: UpsertUser :one
INSERT INTO users (user_id, email, display_name, created_at, last_login_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id) DO UPDATE SET
  email = EXCLUDED.email,
  display_name = EXCLUDED.display_name,
  last_login_at = EXCLUDED.last_login_at,
  updated_at = EXCLUDED.updated_at
RETURNING user_id, email, display_name, created_at, last_login_at, updated_at;

-- name: ListUserWorkspaces :many
SELECT w.workspace_id, w.name, w.created_at
FROM workspaces w
JOIN account_users au ON au.account_id = w.account_id
WHERE au.user_id = $1
ORDER BY w.created_at DESC;
