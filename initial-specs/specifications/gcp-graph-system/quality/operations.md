# Operations

## Error Handling

### Failure Types

- アップロード失敗
- テキスト抽出失敗
- Gemini 呼び出し失敗
- 正規化ツール生成失敗
- サンドボックス実行失敗
- JSON parse 失敗
- BigQuery 書き込み失敗
- RPC ハンドラ失敗
- 非同期ジョブ起動失敗

### Failure Policy

- `documents.status` を `failed` に更新する
- 失敗理由をログに記録する
- 再処理可能な設計とする
- `StartProcessing` は upload 完了済みかつ `documents.status=uploaded` の document のみ受け付ける
- upload 未完了や不正な状態遷移はジョブを起動せず、同期エラーとして返す
- Gemini の返却 JSON が不正な場合は JSON repair を 1 回だけ試行する
- JSON repair 後も不正な場合、同一入力に対する Gemini 再試行を最大 2 回まで行う
- chunk 単位の抽出で 1 chunk でも確定失敗した場合、その document の処理全体を `failed` にする
- HTML サマリ生成の失敗は document 全体の失敗にせず、該当 node の `summary_html=null` で継続する

## Discord Webhook Notifications

Discord の Incoming Webhook を使って管理者チャンネルに通知する。Webhook URL は GCP Secret Manager に保存する。

### 通知イベント一覧

| イベント | トリガー | メッセージ例 |
| --- | --- | --- |
| 人間レビュー必要 | LLM スコア < 0.9 | ⚠️ 正規化ツールのレビューが必要 / ツール名・スコア・理由・リンク |
| ツール自動承認 | LLM スコア ≥ 0.9 | ✅ 正規化ツールが自動承認されました / ツール名・スコア |
| ツール手動承認・廃止 | 管理者が操作 | 👤 ツールが手動承認/廃止されました / ツール名・操作者 |
| document 処理失敗 | `status=failed` | ❌ ドキュメント処理が失敗 / ファイル名・エラーステージ・リンク |
| 評価指標劣化 | 週次評価で 5% 以上低下 | 📉 評価指標が劣化しています / 指標名・前週比・差分 |
| Gemini コスト閾値超過 | 日次コストが設定額超過 | 💸 Gemini コストが閾値を超えました / 当日コスト・閾値 |
| Cloud Run エラー率急上昇 | 直近5分のエラー率が閾値超過 | 🔥 エラー率が急上昇しています / エラー率・閾値 |

### 実装方針

- Cloud Run バックエンドから直接 Discord Webhook URL に HTTP POST する
- コスト・エラー率監視は Cloud Monitoring のアラートポリシーから Cloud Functions 経由で POST する
- Webhook URL は `DISCORD_WEBHOOK_URL` として Secret Manager に保存する

## Monitoring

### Initial Monitoring

- `Cloud Logging` に API 実行ログを出力する
- document 単位で処理開始、成功、失敗を記録する
- RPC メソッド単位でレイテンシと失敗率を記録する
- job 単位で開始、完了、失敗を記録する
- ツール生成、dry-run、本実行のイベントを記録する

### Future Monitoring

- 処理時間監視
- エラー率監視
- Gemini 失敗率監視
- コスト監視

## Security and Access Control

### Initial Policy

- Cloud Storage バケットは非公開
- API 経由でのみファイルアクセス
- サービス間アクセスは GCP IAM により制御

### Future Policy

- ユーザー認証導入
- ドキュメント単位のアクセス制御
- 監査ログの強化

## Authentication Policy

- **Firebase Auth + Google OAuth** を採用する（Firebase Hosting との親和性が高い）
- フロントエンドで Google ログインを要求し、ID トークンを Connect RPC のヘッダに付与する
- バックエンドでトークンを検証し、未認証リクエストを拒否する
- アクセス制御は workspace + role ベースで行う（詳細は下記）

### Workspace とロール

- ユーザーは1つ以上の workspace に所属する
- workspace 内のロールは `editor` / `viewer` / `dev` の3種
- `/dev/stats` は Firebase Auth カスタムクレーム `role: "dev"` を持つユーザーのみアクセス可能

| ロール | 権限 |
| --- | --- |
| `editor` | ドキュメントのアップロード・削除・処理実行・メンバー招待 |
| `viewer` | グラフ閲覧のみ |
| `dev` | `/dev/stats` アクセス（editor/viewer に追加付与） |

## Billing and Plans

### プラン構成

詳細な制限値は [data-model.md](../domain/data-model.md) の `plans` テーブルを参照。

| | free | pro |
| --- | --- | --- |
| ストレージ | 1GB | 50GB |
| ファイルサイズ | 50MB | 500MB |
| アップロード/日 | 10件 | 200件 |
| メンバー数 | 3人 | 20人 |
| extraction_depth | `summary` のみ | `full` + `summary` |

### 制限の適用方針

- `GetUploadURL` RPC でファイルサイズとアップロード数を事前チェックする
- `StartProcessing` RPC で `extraction_depth` がプランで許可されているか確認する
- `InviteMember` RPC でメンバー数上限を確認する
- 制限値は `plans` テーブルから動的に取得する（ハードコードしない）

### Stripe 課金実装

#### フロー

```
[フロント: プランアップグレードボタン]
    ↓
[バックエンド: Stripe Checkout セッション作成]
    ↓
[Stripe ホスト画面でカード入力・決済]
    ↓
[Stripe → POST /billing/webhook]
    ↓
[署名検証 → workspaces.plan 更新]
```

ダウングレード・解約は Stripe Customer Portal をそのまま利用する（UI を自前で作らない）。

#### バックエンドエンドポイント

| エンドポイント | 処理 |
| --- | --- |
| `POST /billing/checkout` | Stripe Checkout セッション作成 → URL を返す |
| `POST /billing/portal` | Stripe Customer Portal セッション作成 → URL を返す |
| `POST /billing/webhook` | Stripe Webhook 受信・署名検証・plan 更新 |

#### 受信する Webhook イベント

| イベント | 処理 |
| --- | --- |
| `checkout.session.completed` | `workspaces.plan` を `pro` に更新、`stripe_subscription_id` を保存 |
| `customer.subscription.updated` | plan を Stripe の状態と同期 |
| `customer.subscription.deleted` | `workspaces.plan` を `free` に戻す |

#### セキュリティ

- Webhook は `stripe-signature` ヘッダを必ず検証する
- plan の変更はフロントから直接できない、必ず Webhook 経由
- Stripe secret key・webhook signing secret は Secret Manager に保存する
- `stripe_customer_id` は workspace 作成時（または初回 checkout 時）に採番して保存する
