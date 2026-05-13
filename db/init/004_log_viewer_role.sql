-- log-viewer 用 read-only ロールと view
--
-- log-viewer は API 経由ではなく Postgres を直接参照する。
-- アプリケーション層の認可を介さないため、DB レイヤで read のみに縛る。
-- 詳細: docs/improvements/log-viewer-direct-db.md

-- ロール作成 (パスワードは環境変数で配るので空でも CREATE は通す)
-- 本番では SQL 適用後に `ALTER ROLE log_viewer WITH PASSWORD '...'` で更新する想定
CREATE ROLE log_viewer LOGIN;

-- log-viewer が参照するテーブル
GRANT SELECT ON document_processing_jobs TO log_viewer;
GRANT SELECT ON job_logs TO log_viewer;
GRANT SELECT ON workspaces TO log_viewer;
GRANT SELECT ON documents TO log_viewer;

-- write は明示的に拒否
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON document_processing_jobs FROM log_viewer;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON job_logs FROM log_viewer;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON workspaces FROM log_viewer;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON documents FROM log_viewer;

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

GRANT SELECT ON v_job_logs TO log_viewer;

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

GRANT SELECT ON v_processing_jobs TO log_viewer;
