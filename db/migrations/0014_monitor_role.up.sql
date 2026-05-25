-- monitor 用 read-only ロールと view
--
-- monitor は API 経由ではなく Postgres を直接参照する。
-- アプリケーション層の認可を介さないため、DB レイヤで read のみに縛る。

-- ロール作成 (パスワードは環境変数で配るので空でも CREATE は通す)
-- 本番では SQL 適用後に `ALTER ROLE monitor WITH PASSWORD '...'` で更新する想定
-- ロールはスキーマ外のクラスタレベルオブジェクトのため、DROP SCHEMA では
-- 消えない。reset-db 等での再実行に耐えるよう IF NOT EXISTS で冪等化する。
CREATE ROLE IF NOT EXISTS monitor LOGIN;

-- monitor が参照するテーブル
GRANT SELECT ON document_processing_jobs TO monitor;
GRANT SELECT ON job_logs TO monitor;
GRANT SELECT ON workspaces TO monitor;
GRANT SELECT ON documents TO monitor;

-- write は明示的に拒否。
-- CockroachDB は TRUNCATE を独立した privilege として扱わない (DELETE 権限
-- に含まれる) ため列挙しない。INSERT/UPDATE/DELETE を REVOKE すれば
-- 実質 TRUNCATE も封じられる。
REVOKE INSERT, UPDATE, DELETE ON document_processing_jobs FROM monitor;
REVOKE INSERT, UPDATE, DELETE ON job_logs FROM monitor;
REVOKE INSERT, UPDATE, DELETE ON workspaces FROM monitor;
REVOKE INSERT, UPDATE, DELETE ON documents FROM monitor;

-- センシティブカラムを含むテーブルは view 経由で公開する。
-- v_job_logs: job_logs から直接公開してよい列をフィルタする (現状は全列)
CREATE VIEW v_job_logs AS
  SELECT
    id,
    job_id,
    workspace_id,
    document_id,
    level,
    event,
    message,
    detail_json,
    created_at
  FROM job_logs;

GRANT SELECT ON v_job_logs TO monitor;

-- v_processing_jobs: 課金や internal flags を含まない最小のジョブ一覧
CREATE VIEW v_processing_jobs AS
  SELECT
    job_id,
    document_id,
    workspace_id,
    job_type,
    status,
    current_stage,
    error_message,
    retry_count,
    created_at,
    updated_at
  FROM document_processing_jobs;

GRANT SELECT ON v_processing_jobs TO monitor;

-- BI ダッシュボード用 view
-- usage_events: PII なし (account_id は ID のみ)
CREATE VIEW v_usage_events AS
  SELECT
    event_id,
    account_id,
    workspace_id,
    job_id,
    model,
    input_tokens,
    output_tokens,
    cost_minor,
    currency,
    paid_via,
    credit_amount_minor,
    stripe_amount_minor,
    created_at
  FROM usage_events;

GRANT SELECT ON usage_events TO monitor;
GRANT SELECT ON v_usage_events TO monitor;

-- account_usage_daily: 日次ロールアップ (PII なし)
CREATE VIEW v_account_usage_daily AS
  SELECT
    account_id,
    usage_date,
    model,
    input_tokens,
    output_tokens,
    cost_minor,
    event_count,
    updated_at
  FROM account_usage_daily;

GRANT SELECT ON account_usage_daily TO monitor;
GRANT SELECT ON v_account_usage_daily TO monitor;

-- v_workspaces: workspace 活動量に使う (name は管理目的で公開)
CREATE VIEW v_workspaces AS
  SELECT
    workspace_id,
    name,
    created_at,
    updated_at
  FROM workspaces;

GRANT SELECT ON v_workspaces TO monitor;

-- v_documents: uploaded_by (ユーザー識別子) を除外
CREATE VIEW v_documents AS
  SELECT
    document_id,
    workspace_id,
    filename,
    mime_type,
    file_size,
    created_at
  FROM documents;

GRANT SELECT ON documents TO monitor;
GRANT SELECT ON v_documents TO monitor;

-- v_tree_items: コンテンツ列を除外してカウント用途のみ公開
CREATE VIEW v_tree_items AS
  SELECT
    id,
    workspace_id,
    parent_id,
    level,
    created_at,
    updated_at
  FROM tree_items;

GRANT SELECT ON tree_items TO monitor;
GRANT SELECT ON v_tree_items TO monitor;

-- v_job_mutation_logs: ツール呼び出しトレース用 (PII なし)
CREATE VIEW v_job_mutation_logs AS
  SELECT
    mutation_id,
    job_id,
    workspace_id,
    target_type,
    target_id,
    mutation_type,
    before_json,
    after_json,
    provenance_json,
    created_at
  FROM job_mutation_logs;

GRANT SELECT ON job_mutation_logs TO monitor;
GRANT SELECT ON v_job_mutation_logs TO monitor;
