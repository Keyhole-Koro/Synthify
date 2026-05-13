# Usage-Based Billing (従量課金)

## Objective
現行の Free / Pro 月額モデルから、LLM API 使用量に応じた従量課金モデルへ移行するための設計と必要コンポーネントを定義する。

## Motivation
- ドキュメント抽出パイプラインの LLM コストはジョブ規模で大きく変動する。
- 月額固定では heavy user の赤字／ light user の過払いが発生する。
- ユーザー側も「使った分だけ払う」モデルを期待するケースが多い。

## Plan Lineup

### Free プラン
- 月次 LLM 使用量に hard cap (例: USD 1.00 相当 / 月)。
- GCS アップロード:
  - 1ファイルあたりの最大サイズ: 小さめ (例: 10 MB)
  - account 全体のストレージ上限: 既存 quota を流用 (例: 100 MB)
- 上限到達後は新規ジョブ submit 不可。

### Usage-Based プラン (従量課金)
- 月額固定費なし、または最低料金 + 従量。
- 使用量に応じて Stripe Meters 経由で月末請求。
- GCS アップロード:
  - 1ファイルあたりの最大サイズ: 大きめ (例: 500 MB / 1 GB)
  - account ストレージ上限: 拡張 or 無制限
- ユーザー設定の `budget_limit_micros` で月次予算を任意設定可能。

## Required Components

### 1. LLM 使用量の計測 (Usage Metering)
- Worker 側で各 LLM 呼び出しの `input_tokens` / `output_tokens` をレスポンスから取得する。
- モデル別の単価テーブル (例: `gemini-2.5-pro` の input/output USD per 1M tokens) を保持する。
- 呼び出しごとに `cost_micros` を算出する。
- 新規テーブル `usage_events` を導入:
  - `event_id`, `account_id`, `workspace_id`, `job_id`, `model`, `input_tokens`, `output_tokens`, `cost_micros`, `created_at`
- 集計用ロールアップ: `account_usage_daily` (account_id, date, total_cost_micros) をマテビュー or 日次バッチで生成。

### 2. GCS アップロードサイズ制限
- `accounts.max_file_size_bytes` を plan に応じて設定 (Free: 10 MB、Usage-Based: 500 MB〜1 GB)。
- 署名付き URL 発行時にプラン由来の上限を埋め込む (`X-Goog-Content-Length-Range` ヘッダ等)。
- API 側でも `signed URL 発行 RPC` で plan を確認し、上限超過の指定なら拒否。
- フロントエンドはアップロード前にファイルサイズを確認しユーザーに即フィードバック。
- account 全体のストレージ上限 (`storage_quota_bytes`) と並行して、1ファイルサイズ上限も plan に紐付ける。

### 3. 予算アラート / 上限
- `accounts` に以下を追加:
  - `budget_limit_micros` — ユーザー設定の上限 (null なら無制限)
  - `current_period_usage_micros` — 当月累計 (period rollover 必要)
  - `budget_exceeded` — boolean フラグ
- 閾値ロジック:
  - 80% 到達でメール通知 (account.email 宛)
  - 100% 到達で `budget_exceeded = true` を立てる
- ゲート箇所:
  - API: 新規ジョブ submit 時に `budget_exceeded` を確認し拒否
  - Worker: ジョブ enqueue / dequeue 時に再確認

### 4. Worker Graceful Shutdown (予算到達時の途中打ち切り)
- 予算到達は「即停止」ではなく「途中成果物で tree を生成して終わらせる」方針とする。
  - 単純な kill だとユーザーは課金されたのに何の出力も得られない、という最悪のUXになる。
  - 抽出済みの concept / claim / evidence までで構成された部分的な tree でも価値がある。
- 設計案:
  - Worker は各 LLM コール直前に予算残量 (or `budget_exceeded` フラグ) を確認。
  - 上限到達を検知したら、現在のステージを中断して **synthesis ステージへフォールバック** する。
    - 既に抽出済みの items を入力に、`synthesis` ツールで簡易 tree を生成。
    - synthesis 自体の LLM コール 1〜2 回分は予算オーバーしても許容 (grace budget)。
  - ジョブ状態は `cancelled` ではなく `completed_partial` (or `truncated_by_budget`) として記録。
  - 出力 tree には「予算到達により部分的に抽出されました」のメタフラグを付与し、UI で表示。
- 「即停止」が必要なケース (admin による緊急停止、決済失敗など) は別経路で:
  - Redis pub/sub で `force_cancel:{account_id}` を発火、synthesis フォールバックもスキップして cancelled に。

### 5. Stripe 連携 (Metered Billing)
- 方針A: Stripe Billing Meters を使う
  - `usage_events` をそのまま `stripe.billing.meterEvents.create` に流す
  - Stripe が請求金額を計算・自動請求
- 方針B: 自前で集計し、Stripe には月次合計だけ送る
  - 制御しやすいが invoice 生成・税計算を自前で持つ必要あり
- 推奨は方針A (Stripe Meters)。

### 6. UI / 可視化
- billing paper に当月使用量・予算進捗バー・モデル別内訳を表示。
- ジョブ submit 前に「想定コスト見積もり」を表示 (ドキュメントサイズ × 推定 token 換算)。
- 履歴ページ: 日次グラフ、ジョブ単位の cost breakdown。

## Open Questions
- **粒度**: コール単位の usage_events は 1 ジョブで数百行になる。集計コストとのバランスをどう取るか。
- **停止レイテンシ**: 「次の LLM コール前にチェック」までは最大1コール分は課金される。これを許容するか、stream cancel まで実装するか。
- **Grace budget の大きさ**: synthesis フォールバック時に許容する追加課金は何 micros までか。固定額か、予算の N% か。
- **抽出途中の最小成果物**: どのステージまで進めば「synthesis に渡せる items」が揃うか。chunk 分割段階で予算到達した場合の挙動は？
- **見積もり精度**: 事前見積もりの誤差をどこまで許容するか。実コストとの diff をユーザーにどう説明するか。
- **既存 Pro プランとの併存**: 月額 (容量制) と 従量 (LLM 使用量制) を両立させるか、完全移行するか。

## Migration Strategy
1. **Phase 1**: usage_events の記録のみ追加 (課金には反映しない、shadow mode)
2. **Phase 2**: Free / Usage-Based プラン定義 + GCS アップロードサイズ制限の plan 連動
3. **Phase 3**: 予算アラート機能を追加 (notification のみ、ゲートなし)
4. **Phase 4**: budget_exceeded のゲート + Worker graceful shutdown (synthesis フォールバック)
5. **Phase 5**: Stripe Meters 連携 / 請求書発行

## References
- [stripe-billing-integration.md](stripe-billing-integration.md) — 月額モデルの契約定義
- [billing-revenue-operations-audit.md](billing-revenue-operations-audit.md) — 既存課金まわりの監査
