-- Seed data for development
-- model_pricing: Gemini 2.5 Flash 公式料金 (2025-05) + display_multiplier
-- input: $0.30/1M tokens = 30 cents/1M = 30 minor/1M
-- output: $2.50/1M tokens = 250 cents/1M = 250 minor/1M
-- ドキュメントの実体は fake-gcs-server に置く。
-- scripts/seed_gcs.sh でアップロード後、scripts/worker_demo.sh でジョブを実行する。

INSERT INTO accounts (account_id, name, plan, created_at, updated_at)
VALUES ('acc_seed_1', 'Demo Account', 'pro', NOW(), NOW())
ON CONFLICT (account_id) DO NOTHING;

INSERT INTO account_users (account_id, user_id, role, joined_at)
VALUES ('acc_seed_1', 'user_seed_1', 'admin', NOW())
ON CONFLICT (account_id, user_id) DO NOTHING;

INSERT INTO workspaces (workspace_id, account_id, name, created_at, updated_at)
VALUES
  ('ws_seed_1', 'acc_seed_1', 'Platform Engineering', NOW(), NOW()),
  ('ws_seed_2', 'acc_seed_1', 'Medical Research',     NOW(), NOW())
ON CONFLICT (workspace_id) DO NOTHING;

-- doc_llm_1: Go マイクロサービス設計書 (英語)
-- doc_llm_2: 臨床試験プロトコル (日本語)
-- doc_llm_3: レガシー移行メモ (英語・技術負債)
INSERT INTO documents (document_id, workspace_id, uploaded_by, filename, mime_type, file_size, created_at)
VALUES
  ('doc_llm_1', 'ws_seed_1', 'user_seed_1', 'microservices_design.md',    'text/markdown', 4200, NOW()),
  ('doc_llm_2', 'ws_seed_2', 'user_seed_1', 'clinical_trial_protocol.md', 'text/markdown', 3800, NOW()),
  ('doc_llm_3', 'ws_seed_1', 'user_seed_1', 'legacy_migration_notes.md',  'text/markdown', 2900, NOW())
ON CONFLICT (document_id) DO NOTHING;

INSERT INTO model_pricing
  (model, input_cost_per_mtoken_minor, output_cost_per_mtoken_minor, currency, display_multiplier, effective_from, notes)
VALUES
  ('gemini-2.5-flash-preview-05-20', 30,  250, 'usd', 1.0, NOW(), 'Gemini 2.5 Flash — baseline x1'),
  ('gemini-2.5-flash',               30,  250, 'usd', 1.0, NOW(), 'Gemini 2.5 Flash stable'),
  ('gemini-2.5-pro-preview-05-06',   250, 1500,'usd', 5.0, NOW(), 'Gemini 2.5 Pro — x5'),
  ('gemini-2.5-pro',                 250, 1500,'usd', 5.0, NOW(), 'Gemini 2.5 Pro stable — x5'),
  ('gemini-2.0-flash',               10,  40,  'usd', 1.0, NOW(), 'Gemini 2.0 Flash — x1'),
  ('gemini-3-flash-preview',         30,  250, 'usd', 1.0, NOW(), 'Gemini 3 Flash preview — x1'),
  -- Default worker model (GEMINI_MODEL). Image input is billed as prompt
  -- tokens, so no separate image dimension is needed — but the row must
  -- exist or usage records at cost 0 (see usage.Recorder: unknown model =>
  -- cost 0 + warn). Rate mirrors the Flash tier; confirm against official.
  ('gemini-3.5-flash',               30,  250, 'usd', 1.0, NOW(), 'Gemini 3.5 Flash (worker default) — x1; rate TBD-confirm'),
  ('gemini-3-pro-preview',           250, 1500,'usd', 5.0, NOW(), 'Gemini 3 Pro preview — x5; rate TBD-confirm')
ON CONFLICT (model) DO UPDATE SET
  input_cost_per_mtoken_minor  = EXCLUDED.input_cost_per_mtoken_minor,
  output_cost_per_mtoken_minor = EXCLUDED.output_cost_per_mtoken_minor,
  display_multiplier           = EXCLUDED.display_multiplier,
  notes                        = EXCLUDED.notes;
