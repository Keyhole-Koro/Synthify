# log-viewer の Postgres 直接参照化

## Objective
log-viewer を API 経由ではなく、read-only ロールで Postgres を直接参照する形に切り替える。
これにより API 側から `AnonymousReadAllowed` 機構一式を削除し、認可ロジックを簡潔にする。

## Motivation
現状、log-viewer は API の `JobService` を anonymous で叩く構造になっている。
そのために以下の穴あけ機構が存在する:

- `middleware.WithAuth` の `enableAnonymous` 分岐 (`cfg.Env == "local"` のときだけ有効)
- `middleware.isAnonymousPathAllowed` の許可リスト
  - `/health`
  - `JobService/ListAllJobs`
  - `JobService/ListJobLogs`
  - `JobService/SearchJobLogs`
  - `JobService/ListRelatedJobLogs`
- handler 側の `if middleware.AnonymousReadAllowed(ctx) { return nil }` 分岐
  - `authorizeWorkspace`
  - `authorizeDocument`
  - `authorizeItem`
  - `authorizeAndLoadJob`
  - `ListAllJobs`

これらは「ローカル開発でだけ穴を開ける」設計だが、本番でもコード上は分岐が常に評価されるため、レビュー漏れで誤って許可リストに RPC を追加すると認可スキップが発生する。

## Design

### log-viewer 側
- 新しい環境変数 `LOG_VIEWER_DATABASE_URL` で Postgres に直接接続する。
- 既存の Connect RPC クライアントを廃止し、`pgx` または `database/sql` で SQL を発行。
- log-viewer に必要なテーブル / view にだけアクセスする。

### Postgres 側
read-only ロールと view を作る。例:

```sql
CREATE ROLE log_viewer LOGIN PASSWORD '...';

-- 直接 SELECT を許可するテーブル
GRANT SELECT ON document_processing_jobs TO log_viewer;
GRANT SELECT ON job_logs TO log_viewer;

-- センシティブカラムを除外したい場合は view 経由
CREATE VIEW v_job_logs AS
  SELECT id, job_id, workspace_id, document_id, level, event, message, detail_json, created_at
  FROM job_logs;
GRANT SELECT ON v_job_logs TO log_viewer;

REVOKE INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public FROM log_viewer;
```

DDL は `db/init/008_log_viewer_role.sql` のような新ファイルに追加。

### API 側 (削除対象)
以下を完全削除する:

- `packages/shared/middleware/auth.go`
  - `enableAnonymous` 引数
  - `anonymousReadAllowedContextKey`
  - `AnonymousReadAllowed`
  - `isAnonymousPathAllowed`
- `apps/api/cmd/server/main.go`
  - `WithAuth(cfg.FirebaseProjectID, cfg.Env == "local", ...)` の第2引数
- `apps/api/internal/handler/authz.go` の3箇所の `if middleware.AnonymousReadAllowed(ctx)` 分岐
- `apps/api/internal/handler/job.go` の `ListAllJobs` と `authorizeAndLoadJob` の anonymous 分岐
- 関連テスト (anonymous read を assert していたもの)

## Migration Strategy
1. **Phase 1**: read-only ロールと view を DB 側に追加 (`db/init/008_log_viewer_role.sql`)
2. **Phase 2**: log-viewer を pg 直接接続に書き換え、ローカルで動作確認
3. **Phase 3**: API 側の anonymous 機構を削除、handler テストを更新
4. **Phase 4**: 本番デプロイ。log-viewer のホスティング先から Postgres への接続経路 (Cloud SQL Auth Proxy など) を整備

## Trade-offs

### Pro
- API の認可ロジックが「ログイン or サービストークン or 拒否」の3択で完結する
- 「許可リスト追加で認可スキップ」のレビュー漏れリスクが消える
- DB レイヤの GRANT で write を物理的に拒否できる
- log-viewer 用クエリの自由度が上がる (API に RPC を生やさず SQL で完結)

### Con
- log-viewer がスキーマカラム名に直接結合する (proto 経由の抽象化が消える)
  - スキーマ変更時は view 定義を合わせて変更する必要あり
- log-viewer のホスティング場所から Postgres への接続経路が必要
  - Cloud SQL Auth Proxy / SSH tunnel / VPN など
- secret 管理が増える (`LOG_VIEWER_DATABASE_URL`)

## Related
- [handler-auth-test-coverage.md](handler-auth-test-coverage.md) — anonymous-read context のテストを残作業として記載中。この PR で削除する場合はそのまま消える。
