# Stripe Billing Contract

## Objective
Stripe 導入前に、課金まわりの責務分界とデータ契約を固定する。

この文書は「どう実装するか」より先に、「何を source of truth にするか」「どの ID を API に渡すか」「plan をどう表現するか」を定義するためのものとする。

## Current Constraints
- 既存の永続化モデルでは `accounts` が quota と `plan` を保持している。
- `workspaces` は `account_id` に従属しており、課金主体ではない。
- 一方で既存の `billing.proto` は `workspace_id` を受けるため、現行 proto は契約として不正確である。
- 既存コードでは account 作成時の `plan` に `registered` を入れているが、公開 proto の `WorkspacePlan` は `FREE` / `PRO` しか持たない。

## Contract

### 1. Billing Owner
- 課金主体は `workspace` ではなく `account` とする。
- 1つの Stripe Customer は 1つの `account` にのみ対応する。
- 1つの `account` 配下に複数 `workspace` が存在しても、課金状態は共有される。
- `workspace` は billing state を直接保持しない。表示用に `account` の plan / quota を投影するだけとする。

### 2. Source Of Truth
- 課金状態の source of truth はローカル DB の `accounts` テーブルとする。
- Stripe は外部決済システムであり、決済イベントの入力元ではあるが、アプリケーションの認可判定は常にローカル DB を参照する。
- webhook を受けるまでは UI 上の optimistic な成功表示をしてもよいが、機能制限の解除は DB 更新後にのみ行う。

### 3. Plan Semantics
- billing 上の正規 plan 値は `free` と `pro` の2値とする。
- `registered` は課金プランではなく、既存実装由来の暫定値として扱う。
- 移行完了後、`registered` は `free` に統合する。
- API / proto / frontend に公開する plan は `free` / `pro` のみとする。

### 4. Account Schema Contract
`accounts` テーブルは少なくとも次の列を持つものとする。

- `account_id`
- `plan`
- `storage_quota_bytes`
- `storage_used_bytes`
- `max_file_size_bytes`
- `max_uploads_per_5h`
- `max_uploads_per_1week`
- `stripe_customer_id`
- `stripe_subscription_id`
- `updated_at`

追加ルール:

- `stripe_customer_id` は account ごとに高々1つ。
- `stripe_subscription_id` は現在有効な subscription を指す。free のときは空でよい。
- quota 列は plan の派生値だが、実行時高速化のため denormalized に保持してよい。

### 5. API Identity Contract
- billing API の主語は `account_id` とする。
- `workspace_id` を受ける billing API は新規追加しない。
- 既存 UI が workspace 文脈で billing を開く場合でも、backend で `workspace_id -> account_id` を解決してから内部 billing サービスを呼ぶ。
- proto の public contract では `CreateCheckoutSessionRequest` / `CreatePortalSessionRequest` は最終的に `account_id` を受ける形へ寄せる。

### 6. Access Control Contract
- checkout session / portal session 作成は、その `account` のメンバーのみ許可する。
- subscription の管理操作は最低でも account owner に制限する。
- webhook は user session と無関係に受信し、Stripe 署名検証のみを信頼境界とする。

### 7. Stripe Object Mapping
- Stripe Customer <-> `accounts.stripe_customer_id`
- Stripe Subscription <-> `accounts.stripe_subscription_id`
- Stripe Price ID <-> 内部の plan 定義 (`pro`)

補足:

- 初期導入では有料プランは `pro` のみとする。
- 将来 `team` や usage-based billing を追加する場合も、`account` 主体の契約は維持する。

### 8. Webhook Contract
必須で扱うイベントは次の通り。

- `checkout.session.completed`
- `invoice.payment_succeeded`
- `invoice.payment_failed`
- `customer.subscription.deleted`

イベント処理契約:

- webhook 処理は冪等であること。
- 各イベントは Stripe object ID をキーに重複処理を防げること。
- `invoice.payment_succeeded` で `pro` を維持する。
- `invoice.payment_failed` は即時解約と同義にしない。別途 grace period を導入するまでは、少なくとも自動で destructive なダウングレードをしない。
- `customer.subscription.deleted` を受けた時点で `free` に戻す。

### 9. Enforcement Contract
free tier の制限判定は、少なくとも次の2箇所で必須とする。

- ドキュメントアップロード前
- 高コストな LLM 実行開始前

判定原則:

- UI 上の非活性化だけでは不十分。backend で必ず再検証する。
- 判定は `workspace` ではなく、その背後の `account` に対して行う。

### 10. UI Contract
- UI は billing 状態を workspace 固有情報としてキャッシュしない。
- 表示上は workspace settings から遷移してもよいが、実際に表示する plan / quota は account ベースの値とする。
- payment 完了直後の UI は webhook 反映待ち状態を表現できること。
- customer portal へ遷移する導線は、課金状態の編集 UI をアプリ内に再実装しない。

## Migration Rules

### Phase 0
- `registered` を `free` と同義に扱う adapter を backend に置く。
- 既存 proto / mapper / UI 上で `registered` を外に漏らさない。

### Phase 1
- `accounts` に Stripe カラムを追加する。
- sqlc query と domain model を更新する。
- billing API の主語を `account_id` に揃える。

### Phase 2
- webhook で `accounts.plan` と quota を同期する。
- upload / processing 制限を backend で強制する。

### Phase 3
- 必要なら `registered` を DB から完全削除する。

## Explicit Non-Goals
- 初期導入で複数有料プランを同時に扱うこと
- usage-based billing を最初から入れること
- workspace ごとの個別課金
- アプリ内でカード管理 UI をフルスクラッチ実装すること

## Open Follow-Ups
- `billing.proto` を `workspace_id` から `account_id` へ変更する。
- `WorkspacePlan` と `accounts.plan` の対応づけを mapper で明文化する。
- `invoice.payment_failed` 時の grace period を採用するか決める。
- quota 値をコード定数で持つか、別テーブルに切り出すか決める。
