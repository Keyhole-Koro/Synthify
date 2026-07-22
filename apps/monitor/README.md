# Synthify Monitor

Synthify の内部運用ツール。ジョブログ閲覧と運用 BI / LLM Eval ダッシュボードを提供する。

## 構成

- **`joblog/`** (Go) — ジョブログの共有インターフェース (`Event` / `Logger` / `Repository`)。
  worker・api から `context` 経由で使われ、構造化ログを Postgres に永続化する。
- **`ui/`** (Next.js) — monitor の画面本体。BFF パターンで、クライアントは API
  (Connect/gRPC) を叩かず、同一 Next.js アプリの Route Handlers (`/api/...`) が
  サーバーサイドで Postgres を **read-only の `monitor` ロール**で直接参照する。

## UI の機能

| 画面 | 内容 |
| --- | --- |
| Logs | ジョブログの閲覧・検索・関連ジョブ・trace |
| Job Health | 成功率 / avg・p50・p95 処理時間 / ステージ別失敗 / 失敗ファイル種別 / エラーメッセージ / リトライ多発ジョブ / 直近失敗のドリルダウン |
| Cost & Usage | 総コスト・credit/stripe 内訳・日次トレンド・モデル別・高コストジョブ top N |
| Workspace Activity | ドキュメント追加ペース・tree_items 作成数・dormant workspace |
| Errors & Alerts | ERROR 頻度時系列・イベント別発生数・直近 5 分のバースト検知 |
| LLM Eval (`/dashboards/eval`) | run / case の pass rate、prompt variant 比較、モデル別 latency・token、直近 run、失敗ケース、slow case、GCS artifact URI |

期間プリセット (今日 / 過去 7 日 / 今月) で集計範囲を切り替えられる。

## 認証・認可

`/api/jobs/*` と `/api/dashboards/*` は Firebase ID トークンによる認証で保護され、
`SYNTHIFY_ADMIN_USER_EMAILS` に含まれる管理者メールのみアクセスできる
(未設定なら fail-closed で全拒否)。API サーバー (`apps/api`) と同じ admin 判定を共有する。

- クライアントはサインイン後、`authFetch` が `Authorization: Bearer <idToken>` を
  付与して各 API を叩く。トークンは Firebase SDK が自動でリフレッシュする。
- 実質的なセキュリティ境界は各ルートの `requireAdmin`。`AuthGate` は UX 上のログイン
  画面で、サインイン (Google / dev はエミュレータのメール・パスワード) 後に
  `/api/auth/me` で admin かどうかを判定してダッシュボードを表示する。

## データソース

BI 用の view と read-only ロールへの GRANT は以下のマイグレーションで定義:

- `db/migrations/0013_monitor_views.up.sql` — `v_processing_jobs`, `v_job_logs`,
  `v_usage_events`, `v_account_usage_daily`, `v_workspaces`, `v_documents`,
  `v_tree_items` など (PII は view 側で遮蔽)
- `db/migrations/0014_monitor_role.up.sql` — `monitor` ロールへの `GRANT SELECT`
- `db/migrations/0022_eval_monitoring.up.sql` — `eval_runs`, `eval_case_results`,
  `v_eval_runs`, `v_eval_case_results` と monitor 用 read-only grant

LLM Eval runner は `DATABASE_DSN` (または `EVAL_DATABASE_DSN`) が設定されている場合、
GCS artifact の出力後に run / case telemetry を同一 transaction で保存する。

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
