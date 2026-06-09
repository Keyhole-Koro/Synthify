-- Workspace members: workspace 単位の共有メンバーシップ
--
-- 既存の account_users は account 単位 (同一 account の全員が全 workspace に
-- アクセスできるフラットモデル)。これとは独立に、workspace 単位で
-- 他ユーザーを招待し role を付与するためのテーブル。
--
-- 認可ゲート IsWorkspaceAccessible はこのテーブルも見るように拡張する。
-- 課金が生じる操作 (処理など) は登録済み (user_id が確定した) editor 以上の
-- メンバーのみに許可し、コストは実行者本人の account に計上する。

CREATE TABLE IF NOT EXISTS workspace_members (
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  user_id      TEXT NOT NULL,
  role         TEXT NOT NULL DEFAULT 'viewer',  -- owner|editor|viewer
  invited_by   TEXT NOT NULL,
  invited_at   TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (workspace_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_workspace_members_user_id ON workspace_members(user_id);
