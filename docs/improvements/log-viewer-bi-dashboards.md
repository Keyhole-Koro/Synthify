# log-viewer の BI ダッシュボード化

## Objective
log-viewer を「ログ閲覧ツール」から「運用 BI」へ拡張する。
ジョブ成功率・LLM コスト・ワークスペース活動量などを可視化し、
日常的な運用判断と異常検知をブラウザで完結させる。

前提として log-viewer は Postgres を read-only ロールで直接参照する
構造に移行済み。
追加クエリは API を経由せず BFF (Next.js Route Handlers) から発行する。

## Motivation
- 現状 log-viewer はジョブログを横串で見るだけで、集計値や時系列を出せない
- ジョブ成功率や失敗ステージを毎回 psql で取るのは効率が悪い
- 従量課金 ([usage-based-billing.md](usage-based-billing.md)) を導入すると、
  当月予算消費・モデル別コスト・予算アラートの状況をすぐ見たい場面が増える
- production で「いま何が遅い / 失敗している」を素早く把握する手段が欲しい

## 想定ダッシュボード

### 1. Job Health
- ジョブ成功率 / 失敗率 (時系列、日次・週次)
- ステージ別の失敗回数 (current_stage × status)
- 平均処理時間 (updated_at - created_at) と p50 / p95
- リトライ多発ジョブの top N

クエリ元: `v_processing_jobs`, `v_job_logs`

### 2. Cost & Usage (従量課金導入後)
- 当月の総コスト (account 別 / model 別)
- 日次コストトレンド
- 予算閾値到達 account 一覧
- ジョブ単位のコスト top N (どのドキュメントが高い?)

クエリ元: `usage_events`, `account_usage_daily`, `model_pricing`

新規 view: `v_account_usage_daily`, `v_usage_events` (account_id / job_id だけ
を公開、PII は遮蔽) を `db/init/004_log_viewer_role.sql` に追加する。

### 3. Workspace Activity
- workspace 別のドキュメント追加ペース (created_at の日次)
- tree_items の作成数 / 編集数
- 最終アクティビティから N 日以上経過した dormant workspace

クエリ元: `workspaces`, `documents`, `tree_items` の read-only view を追加

### 4. Errors & Alerts
- level=ERROR の頻度時系列
- イベント名別の発生回数 (event の top N)
- 直近 N 分のエラーバースト検出

クエリ元: `v_job_logs`

## Implementation Strategy

### Phase 1: 固定ダッシュボード (内製)
- log-viewer に `/dashboards` ページ群を追加
- BFF `app/api/dashboards/*` で集計クエリを発行
- 描画は recharts などの軽量ライブラリ
- 期間プリセット (今日 / 過去7日 / 今月) + workspace フィルタ
- 利点: 既存の log-viewer 認証/ホスティングに乗る、追加サービス無し

### Phase 2: Ad-hoc 分析ツール接続 (検討)
- Metabase / Redash / Grafana のいずれかを Postgres の log_viewer ロールで接続
- 内製ダッシュボードでカバーできない深掘りや、ピボット集計はそちらで
- log-viewer 側はあくまで日常 UX 用、BI 専門ツールは "explorer" として共存

### Phase 3: 集計マテビュー / pre-aggregation
- ダッシュボードクエリが重くなったら、`account_usage_daily` のような
  pre-aggregated テーブルを増やす (ETL は worker か別バッチ)
- リアルタイム性を捨てて latency を稼ぐ

## Open Questions
- **権限の追加グラント**: BI 用途で `workspaces` / `documents` / `tree_items` を
  log_viewer ロールにグラントする際、メールなどの PII は view で除外しないと、
  ロールを抜けば誰でも見られる状態になる。view 設計を最初に固める必要あり。
- **テナント分離**: ダッシュボードは admin 向け前提で全 workspace を横断する。
  個別 workspace owner にも一部公開する場合は別途認可層が要る。
- **誰がアクセスするか**: 現状 log-viewer は内部用。BI を入れるとプロダクトチーム
  / SRE / billing 担当など参照者が増える。誰がどこまで見えるかを Phase 1 着手前に
  整理する。
- **可観測性ツールとの棲み分け**: New Relic などの APM と機能が重複する。BI は
  「ビジネス指標」(コスト、利用状況、収益)、APM は「システム指標」(レイテンシ、
  エラー率)、というスライスで分離するのが現実的。

## References
- [usage-based-billing.md](usage-based-billing.md) — Cost ダッシュボードのデータソース
- [logging.md](logging.md) — Job Health ダッシュボードのデータソースとなる構造化ログ
