-- Workspace share links: 公開リンク共有
--
-- token を知っていれば無認証で workspace を閲覧できるリンク。
-- role は閲覧専用に限定する想定 (課金が生じる操作は招待メンバーのみ)。
-- ResolveShareLink RPC が token から workspace を解決する無認証経路で参照する。
--
-- revoked_at が NULL かつ expires_at が未来 (または NULL=無期限) のものだけが有効。

CREATE TABLE IF NOT EXISTS workspace_share_links (
  token        TEXT PRIMARY KEY,                 -- ランダム不可推測トークン
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  role         TEXT NOT NULL DEFAULT 'viewer',   -- owner|editor|viewer (現状は viewer 運用)
  created_by   TEXT NOT NULL,
  expires_at   TIMESTAMPTZ,                       -- NULL=無期限
  revoked_at   TIMESTAMPTZ,                       -- NULL=有効
  created_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_workspace_share_links_workspace_id ON workspace_share_links(workspace_id);
