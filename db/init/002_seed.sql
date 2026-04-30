-- Seed data for development
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
