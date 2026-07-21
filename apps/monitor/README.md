# Synthify Monitor

Synthify の内部運用ツール。ジョブログ閲覧と運用 BI ダッシュボードを提供する。

## 構成

- **`joblog/`** (Go) — ジョブログの共有インターフェース (`Event` / `Logger` / `Repository`)。
  worker・api から `context` 経由で使われ、構造化ログを Postgres に永続化する。
- **`ui/`** (Next.js) — monitor の画面本体。BFF パターンで、クライアントは API
  (Connect/gRPC) を叩かず、同一 Next.js アプリの Route Handlers (`/api/...`) が
  サーバーサイドで Postgres を **read-only の `monitor` ロール**で直接参照する。

## UI の機能

| タブ | 内容 |
| --- | --- |
| Logs | ジョブログの閲覧・検索・関連ジョブ・trace |
| Job Health | 成功率 / avg・p50・p95 処理時間 / ステージ別失敗 / 失敗ファイル種別 / エラーメッセージ / リトライ多発ジョブ / 直近失敗のドリルダウン |
| Cost & Usage | 総コスト・credit/stripe 内訳・日次トレンド・モデル別・高コストジョブ top N |
| Workspace Activity | ドキュメント追加ペース・tree_items 作成数・dormant workspace |
| Errors & Alerts | ERROR 頻度時系列・イベント別発生数・直近 5 分のバースト検知 |

期間プリセット (今日 / 過去 7 日 / 今月) で集計範囲を切り替えられる。

## 認証・認可

`/api/jobs/*` と `/api/dashboards/*` は Firebase ID トークンによる認証で保護され、
`SYNTHIFY_ADMIN_USER_EMAILS` に含まれる管理者メールのみアクセスできる。
ブラウザからは `/login` で Google サインイン後、httpOnly のセッション Cookie が
発行される。API サーバー (`apps/api`) の認可モデルと同じ admin 判定を共有する。

## データソース

BI 用の view と read-only ロールへの GRANT は以下のマイグレーションで定義:

- `db/migrations/0013_monitor_views.up.sql` — `v_processing_jobs`, `v_job_logs`,
  `v_usage_events`, `v_account_usage_daily`, `v_workspaces`, `v_documents`,
  `v_tree_items` など (PII は view 側で遮蔽)
- `db/migrations/0014_monitor_role.up.sql` — `monitor` ロールへの `GRANT SELECT`

## 環境変数

| 変数 | 用途 |
| --- | --- |
| `MONITOR_DATABASE_URL` | read-only `monitor` ロールを指す Postgres DSN |
| `FIREBASE_PROJECT_ID` | Firebase ID トークン検証に使うプロジェクト ID |
| `SYNTHIFY_ADMIN_USER_EMAILS` | 管理者メールの CSV (api と共通) |
| `NEXT_PUBLIC_FIREBASE_*` | ブラウザ側 Firebase SDK の設定 |
| `FIREBASE_AUTH_EMULATOR_HOST` | dev で Auth エミュレータに接続する場合 |

## ローカル開発

リポジトリ全体の `compose.yaml` で `monitor` サービスとして起動する
(`bun run dev`, port 5174)。詳細はルートの `README.md` / `Makefile` を参照。
