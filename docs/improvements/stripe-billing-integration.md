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

### 8. Application Abstraction Contract
- backend の application layer は Stripe SDK や Stripe event 名に直接依存しない。
- 課金ユースケースは app 固有の `BillingService` に集約し、外部決済事業者との通信は `BillingProvider` interface 越しに行う。
- `BillingProvider` は「将来なんでも差し替え可能な万能 abstraction」ではなく、Synthify が必要とする最小ユースケースだけを表現する。
- 初期導入では provider 実装は Stripe 1つでよい。

配置方針:

- `domain`
- billing 用の app 固有 value object と enum を置く

- `repository`
- `AccountRepository` のような永続化契約だけを置く
- Stripe や billing provider の interface は置かない

- `application service`
- `BillingService` を置く
- account 権限確認と provider 呼び出しの orchestration を担う

- `infrastructure/stripe`
- `BillingProvider` の Stripe 実装を置く
- Stripe SDK、price ID、webhook signature 検証を閉じ込める

責務分界:

- `BillingService`
- account 権限確認
- account / plan / quota の source of truth 管理
- webhook 処理結果の DB 反映
- provider 返却値を app 固有の型へ変換
- `workspace_id -> account_id` 解決

- `BillingProvider`
- checkout session 作成
- customer portal session 作成
- customer / subscription の外部状態取得
- webhook 署名検証と provider payload の正規化
- 外部 object ID と app 固有 plan の対応づけ

アプリ内部で持つべき最小 interface 例:

```go
type BillingProvider interface {
	EnsureCustomer(ctx context.Context, account *domain.Account) (*domain.BillingCustomerRef, error)
	CreateCheckoutSession(ctx context.Context, account *domain.Account, plan domain.BillingPlan) (*domain.BillingCheckoutSession, error)
	CreatePortalSession(ctx context.Context, account *domain.Account) (*domain.BillingPortalSession, error)
	ParseWebhook(ctx context.Context, payload []byte, signature string) (*domain.ProviderWebhookEvent, error)
}
```

```go
type BillingService interface {
	CreateCheckoutSession(ctx context.Context, accountID string, actorUserID string, plan domain.BillingPlan) (*domain.BillingCheckoutSession, error)
	CreatePortalSession(ctx context.Context, accountID string, actorUserID string) (*domain.BillingPortalSession, error)
	HandleWebhook(ctx context.Context, payload []byte, signature string) error
}
```

補足:

- interface 名や戻り値の詳細は実装時に調整してよいが、`StripeCustomer` や `StripeCheckoutSession` のような provider 固有名を application 層へ漏らさない。
- `price_id` は provider 実装内で管理してよく、proto や frontend へ公開しない。
- `checkout.session.completed` のような Stripe 固有 event 名は adapter 内で app 固有 event に変換する。
- `BillingProvider` は repository を知らない。DB 更新は常に `BillingService` 側が行う。

### 8.1 Internal Types Contract
初期導入では少なくとも次の app 固有型を持つ。

```go
type BillingPlan string

const (
	BillingPlanFree BillingPlan = "free"
	BillingPlanPro  BillingPlan = "pro"
)

type BillingCheckoutSession struct {
	URL string
}

type BillingPortalSession struct {
	URL string
}

type BillingCustomerRef struct {
	ExternalCustomerID string
}

type ProviderWebhookEvent struct {
	EventID                string
	EventType              string
	ExternalCustomerID     string
	ExternalSubscriptionID string
	Plan                   BillingPlan
}
```

この型定義は固定 API ではなく設計意図を示すものであり、実装時に最小限の調整は許容する。

### 8.2 Repository Contract For Billing
- 課金実装のために repository interface を新設する場合でも、それは `AccountRepository` の拡張として表現する。
- `BillingRepository` のような Stripe 専用 repository を分離しない。
- repository に追加してよい責務は次に限定する。

- account ごとの billing state 取得
- `stripe_customer_id` / `stripe_subscription_id` 更新
- plan / quota 更新
- webhook 冪等性に必要な永続化

避けるもの:

- repository method に `price_id` を渡すこと
- repository method が Stripe SDK 型を引数や戻り値に持つこと
- repository が webhook payload を直接解釈すること

### 9. Domain Naming Contract
- app 内部で使う課金用語は Stripe 固有語ではなく app 固有語に寄せる。
- ただし外部連携キーとして `stripe_customer_id` / `stripe_subscription_id` を DB に持つことは許容する。

推奨する app 固有型:

- `BillingPlan`
- `BillingState`
- `BillingCheckoutSession`
- `BillingPortalSession`
- `BillingCustomerRef`
- `ProviderWebhookEvent`

避けるもの:

- `StripeCustomer` を domain 型として使うこと
- `StripeSubscription` を repository interface に出すこと
- `price_id` を proto に載せること

### 10. Webhook Contract
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

### 11. Enforcement Contract
free tier の制限判定は、少なくとも次の2箇所で必須とする。

- ドキュメントアップロード前
- 高コストな LLM 実行開始前

判定原則:

- UI 上の非活性化だけでは不十分。backend で必ず再検証する。
- 判定は `workspace` ではなく、その背後の `account` に対して行う。

### 12. UI Contract
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
- `domain` に billing 用 app 固有型を追加する。
- `BillingService` / `BillingProvider` の interface を追加する。
- billing 用 repository 追加が必要なら `AccountRepository` 側に統合する。

### Phase 2
- Stripe adapter を `BillingProvider` 実装として追加する。
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
- `WorkspacePlan` と `accounts.plan` の対応づけを mapper で明文化する。
- Stripe adapter 内で `pro -> price_id` をどう設定注入するか決める。
- webhook 冪等性を `processed_webhook_events` テーブルで持つか、`accounts` 更新だけで吸収するか決める。
- `invoice.payment_failed` 時の grace period を採用するか決める。
- quota 値をコード定数で持つか、別テーブルに切り出すか決める。
