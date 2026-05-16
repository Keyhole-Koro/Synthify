-- Dynamic tools: transform code the LLM generates at runtime.
--   dynamic_tools : 生成ツールの本体・昇格状態・risk tier・監査情報
--
-- 設計: docs/improvements/dynamic-tool-synthesis.md
-- - ジョブ内で status=candidate として記録、tier 判定・昇格はジョブ外の非同期処理
-- - 既定 workspace スコープで誕生、global は明示の追加昇格のみ
-- - レジストリ解決キーは (scope, origin_workspace_id, name, version)
-- - declared/floor/risk の 3 tier を全保存 (申告と機械判定の差を監査可能に)

CREATE TABLE IF NOT EXISTS dynamic_tools (
  tool_id             TEXT PRIMARY KEY,
  name                TEXT NOT NULL,                 -- LLM が呼ぶ論理名
  version             INTEGER NOT NULL,              -- 同一 (scope,ws,name) 内で単調増加
  scope               TEXT NOT NULL DEFAULT 'workspace', -- 'workspace' | 'global'
  origin_workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  origin_job_id       TEXT NOT NULL,

  description         TEXT NOT NULL DEFAULT '',
  language            TEXT NOT NULL,                 -- 'python' | 'sh' | 'starlark'
  code                TEXT NOT NULL,
  io_schema           TEXT NOT NULL DEFAULT '{}',    -- JSON Schema (input + output)

  declared_tier       TEXT NOT NULL DEFAULT 'tier_1',-- LLM 自己申告
  floor_tier          TEXT NOT NULL DEFAULT 'tier_1',-- executor 静的解析の機械的下限
  risk_tier           TEXT NOT NULL DEFAULT 'tier_1',-- 実効値 = max(floor, declared)

  status              TEXT NOT NULL DEFAULT 'candidate', -- candidate|active|held|rejected|disabled

  input_sample        TEXT NOT NULL DEFAULT '',      -- 生成時の動作確認入力 (保全必須)
  runtime_version     TEXT NOT NULL DEFAULT '',      -- 昇格時の executor ランタイム

  use_count           INTEGER NOT NULL DEFAULT 0,    -- エフェメラル含む使用回数
  created_at          TIMESTAMPTZ NOT NULL,
  promoted_at         TIMESTAMPTZ,
  reviewed_by         TEXT NOT NULL DEFAULT ''
);

-- 採番の真実: 同一 (scope, origin_workspace_id, name) で version は一意。
-- 別 workspace は独立した version 系列になる。
CREATE UNIQUE INDEX IF NOT EXISTS uq_dynamic_tools_identity
  ON dynamic_tools(scope, origin_workspace_id, name, version);

-- candidate を古い順に拾う (ジョブ外の昇格処理)。
CREATE INDEX IF NOT EXISTS idx_dynamic_tools_candidates
  ON dynamic_tools(created_at)
  WHERE status = 'candidate';

-- レジストリ解決: そのワークスペースの active + global の active。
CREATE INDEX IF NOT EXISTS idx_dynamic_tools_resolve
  ON dynamic_tools(scope, origin_workspace_id)
  WHERE status = 'active';
