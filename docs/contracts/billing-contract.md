# Billing Contract

このドキュメントは Synthify における課金システムの「契約 (Contract)」を定義する。
LLM の従量課金、無料/購入クレジット、Stripe 連携、Webhook、認可ルールまで、
billing に関わる関数・インターフェイス・データフローを **このドキュメント単体で読み切れる**
レベルで網羅する。実装と乖離した場合はこちらを正とし、コードを直す。

関連:

- 仕様ドラフト: [../improvements/usage-based-billing.md](../improvements/usage-based-billing.md)
- 認可契約: [./api-authorization-contract.md](./api-authorization-contract.md)
- DB スキーマ: [../../db/init/004_billing.sql](../../db/init/004_billing.sql),
  [005_usage_billing.sql](../../db/init/005_usage_billing.sql),
  [008_credits.sql](../../db/init/008_credits.sql),
  [009_usage_events_split.sql](../../db/init/009_usage_events_split.sql)

---

## 1. ドメインモデル

### 1.1 BillingPlan

`packages/shared/domain/billing.go`

| 値 | 意味 | Stripe 顧客 | 課金挙動 |
|---|---|---|---|
| `free` | サインアップ直後の無料プラン | 未登録 | 残高ある限り credit から消費。残高 0 で `CreditStopped` |
| `usage_based` | 従量課金プラン | 登録済 | credit を優先消費し、不足分は Stripe usage-based meter に流す |

これ以外の値は `BillingPlan.Validate()` で `ErrBillingPlanInvalid`。

### 1.2 BillingStatus

| 値 | 意味 |
|---|---|
| `free` | プラン free / Stripe 未登録 |
| `checkout_pending` | Stripe Checkout 開始済み、完了待ち |
| `active` | Stripe subscription/usage が正常 |
| `past_due` | 引き落とし失敗中 |
| `unpaid` | 連続失敗 |
| `canceled` | 解約済 |
| `incomplete` | Checkout 完了直後、Stripe 側で確定待ち |

### 1.3 CreditType

`account_credits.credit_type` の値。残高は `SUM(amount_minor)` で算出する
（行単位の消費は追わない）。

| 値 | amount_minor | 用途 |
|---|---|---|
| `free` | 正 | サインアップ無料付与、admin 手動付与、プロモ |
| `purchased` | 正 | 将来: Stripe Payment Intent で購入したプリペイドクレジット |
| `consumed` | 負 | `RecordUsage` でクレジットから引かれた分。`credit_id="deduct-{event_id}"` |

### 1.4 PaidVia

`usage_events.paid_via` の値。1 件の usage event に対して **必ずどれか 1 つ** が立つ。

| 値 | 条件 | credit_amount_minor | stripe_amount_minor |
|---|---|---|---|
| `credit` | 全額クレジットから消費（Stripe には何も送らない） | = cost_minor | 0 |
| `stripe` | 全額 Stripe usage-based meter 経由 | 0 | = cost_minor |
| `mixed` | 残高を使い切り、超過分が Stripe へ流れた境界 event | balance | cost - balance |

不変条件: `credit_amount_minor + stripe_amount_minor == cost_minor`。

### 1.5 金額表記ルール

- DB の格納値は **すべて minor unit（USD = cents）の `BIGINT`**。
- API/UI 向けには `formatMinor(minor, currency)` で `"10.50"` のような decimal string にする。
  - JPY は小数点なし（`"100"`）。USD は 2 桁固定（`"10.50"`）。
- 入力（`UpdateBudget` 等）も decimal string で受け取り、`parseMinor` で minor に戻す。
- 内部計算は **絶対に float を経由しない**。`computeCostMinor` は整数除算で切り捨て、
  端数分の minor は失われる（割り切れない料率の差は無視できる前提）。

### 1.6 主要型

```go
type UsageEvent struct {
    EventID           string  // 冪等性キー。worker が一意生成。
    AccountID         string
    WorkspaceID       string
    JobID             string
    Model             string  // 例: "gemini-2.5-flash"
    InputTokens       int64
    OutputTokens      int64
    CostMinor         int64   // RecordUsage が pricing から計算してセット
    Currency          string
    CreatedAt         string  // RFC3339
    PaidVia           PaidVia // RecordUsage がセット
    CreditAmountMinor int64   // 同上
    StripeAmountMinor int64   // 同上
}

type UsageRecordResult struct {
    EventID           string
    Cost              string  // decimal string (e.g. "10.50")
    BudgetExceeded    bool    // budget_limit を超えた or CreditStopped
    CreditStopped     bool    // free プランで残高不足、job を止めるべき
    PaidVia           PaidVia
    CreditAmountMinor int64
    StripeAmountMinor int64
}

type ModelPricing struct {
    Model                    string
    InputCostPerMTokenMinor  int64    // minor unit / 1M token
    OutputCostPerMTokenMinor int64
    Currency                 string
    DisplayMultiplier        float64  // UI 表示用: "x1", "x3", "x5" 等
}

type CreditGrant struct {
    CreditID    string     // 冪等性キー
    AccountID   string
    CreditType  CreditType // free / purchased / consumed
    AmountMinor int64      // 正=付与, 負=消費
    Currency    string
    Note        string
    GrantedBy   string     // admin user_id or "system"
    GrantedAt   string     // RFC3339
}
```

### 1.7 定数

```go
domain.FreeSignupCreditMinor int64 = 100  // $1.00 = 100 cents
```

---

## 2. レイヤー構成と呼び出しスタック

```
┌────────────────────────────────────────────────────────────────────┐
│ Connect RPC: BillingService (proto: contracts/.../billing.proto)   │
├────────────────────────────────────────────────────────────────────┤
│ handler/billing.go : BillingHandler                                │
│   - 認証 (currentUser) / 認可委譲 / proto <-> domain mapper        │
├────────────────────────────────────────────────────────────────────┤
│ service/billing.go : BillingUsecase / billingService               │
│   - 認可 (authorizeAccount) / business logic / Stripe orchestration │
├────────────────────────────────────────────────────────────────────┤
│ infrastructure/stripe : BillingProvider 実装                       │
│   - Stripe REST API (checkout, portal, meter, webhook signature)   │
├────────────────────────────────────────────────────────────────────┤
│ repository : AccountRepository + UsageRepository                   │
│   postgres/ (本番) / mock/ (テスト)                                │
├────────────────────────────────────────────────────────────────────┤
│ DB: accounts, account_credits, usage_events, account_usage_daily,  │
│     model_pricing, invoices, payment_methods, billing_webhook_events│
└────────────────────────────────────────────────────────────────────┘
```

---

## 3. インターフェイス

### 3.1 `service.BillingUsecase`

`apps/api/internal/service/billing.go:17`

```go
type BillingUsecase interface {
    GetBillingAccount(ctx, accountID, actorUserID string) (*domain.Account, error)
    CreateCheckoutSession(ctx, accountID, actorUserID, plan, currency) (*BillingCheckoutSession, error)
    CreatePortalSession(ctx, accountID, actorUserID string) (*BillingPortalSession, error)
    HandleWebhook(ctx, payload []byte, signature string) error

    GetUsage(ctx, accountID, actorUserID, periodStart, periodEnd string) (*UsageReport, error)
    RecordUsage(ctx, ev *UsageEvent) (*UsageRecordResult, error)
    UpdateBudget(ctx, accountID, actorUserID, budgetLimit string) (string, error)
    ListInvoices(ctx, accountID, actorUserID string, limit int) (*InvoiceList, error)
    ListPaymentMethods(ctx, accountID, actorUserID string) ([]*PaymentMethod, error)

    // Credits
    GrantFreeSignupCredit(ctx, accountID string) error
    GrantCredit(ctx, actorUserID, accountID string, amountMinor int64, note string) (*CreditGrant, error)
    GetCreditBalance(ctx, accountID, actorUserID string) (int64, error)
}
```

`actorUserID` を取らない `RecordUsage` / `GrantFreeSignupCredit` / `HandleWebhook` は
**内部経路専用** (詳細 §6)。

### 3.2 `service.BillingProvider`

Stripe など外部決済プロバイダの抽象。本番実装は `apps/api/internal/infrastructure/stripe`。

```go
type BillingProvider interface {
    EnsureCustomer(ctx, *Account) (*BillingCustomerRef, error)
    CreateCheckoutSession(ctx, *Account, plan, currency) (*BillingCheckoutSession, error)
    CreatePortalSession(ctx, *Account) (*BillingPortalSession, error)
    ParseWebhook(ctx, payload []byte, signature string) (*ProviderWebhookEvent, error)
    ReportTokenUsage(ctx, *Account, identifier string, inputTokens, outputTokens int64) error
}
```

ルール:

- `ReportTokenUsage` は `account.StripeCustomerID == ""` で no-op、エラーを返さない。
- `identifier` は idempotency key の prefix。`{event_id}:in` / `{event_id}:out` で
  meter ごとに分けて Stripe に送る。
- `nil` 実装が許容される（local dev）。service 側で `provider == nil` をチェックする。

### 3.3 `repository.UsageRepository`

`packages/shared/repository/interfaces.go:86`

```go
type UsageRepository interface {
    GetModelPricing(ctx, model string) (*ModelPricing, error)
    RecordUsageAccounting(ctx, ev *UsageEvent, date string) (string, bool, error)
    ListUsageByModel(ctx, accountID, periodStart, periodEnd string) ([]ModelUsage, string, error)
    ListDailyUsage(ctx, accountID, periodStart, periodEnd, currency string) ([]DailyUsage, error)
    UpdateAccountBudgetLimit(ctx, accountID string, limitMinor int64) error
    ListInvoices(ctx, accountID string, limit int) (*InvoiceList, error)
    ListPaymentMethods(ctx, accountID string) ([]*PaymentMethod, error)

    // Credits
    GrantCredit(ctx, *CreditGrant) error
    GetCreditBalance(ctx, accountID string) (int64, error)
    ListCreditGrants(ctx, accountID string, limit int) ([]*CreditGrant, error)
}
```

`RecordUsageAccounting` 戻り値: `(eventID, budgetExceeded, error)`。
- `eventID` は冪等性確認用（duplicate なら既存 ID を返す）。
- `budgetExceeded == true` は `accounts.budget_limit_minor > 0` かつ累計コストが
  これを超えた瞬間のみ。それ以降の event でも `true` を返し続ける。

### 3.4 `repository.AccountRepository` (billing 関連抜粋)

```go
GetAccount(ctx, accountID) (*Account, error)
SetAccountStripeCustomerID(ctx, accountID, stripeCustomerID) error
ApplyBillingPlan(ctx, accountID, stripeCustomerID, stripeSubscriptionID, plan) error
ApplyBillingPlanByStripeCustomerID(ctx, stripeCustomerID, stripeSubscriptionID, plan) error
RecordBillingWebhookEvent(ctx, *ProviderWebhookEvent) (alreadyProcessed bool, err error)
MarkBillingWebhookEventProcessed(ctx, provider, eventID, status, errorMessage) error
ApplyBillingEvent(ctx, *ProviderWebhookEvent) error
```

---

## 4. Connect RPC 表面

`contracts/connectrpc/synthify/tree/v1/billing.proto`

| RPC | 認可 | 用途 |
|---|---|---|
| `GetBillingAccount` | ユーザー認証 | アカウント全体（plan, status, budget, credit balance, credit_stopped）|
| `CreateCheckoutSession` | ユーザー認証 | Stripe Checkout URL を返す |
| `CreatePortalSession` | ユーザー認証 | Stripe Customer Portal URL を返す |
| `GetUsage` | ユーザー認証 | 期間集計（by model, by day）|
| `RecordUsage` | **service token のみ** | worker からの usage 計測 |
| `UpdateBudget` | ユーザー認証 | 月次予算上限の更新 |
| `ListInvoices` | ユーザー認証 | Stripe invoice キャッシュ |
| `ListPaymentMethods` | ユーザー認証 | Stripe payment method キャッシュ |
| `GrantCredit` | admin user | クレジット手動付与 |
| `GetCreditBalance` | ユーザー認証 | 現残高 |

Webhook は Connect ではなく **生 HTTP** で公開する: `POST /api/v1/billing/webhook`
(`NewBillingWebhookHTTPHandler`)。

---

## 5. データフロー: `RecordUsage` (厳密版 §5 重要)

worker から API への 1 LLM コール分の課金フロー。**この節がシステムの心臓部**。

### 5.1 シーケンス

```
worker LLM call
      │
      ▼
metering.connectReporter.RecordUsage
  (POST /BillingService/RecordUsage with X-Synthify-Service-Token)
      │
      ▼
BillingHandler.RecordUsage
  - middleware.IsServiceCall を必須
  - proto -> domain.UsageEvent
      │
      ▼
billingService.RecordUsage(ev)
  1. validate (EventID/AccountID/Model 必須)
  2. usage == nil なら stub (Stripe meter のみ送って終了)
  3. GetModelPricing(model) -> cost_minor を計算
  4. GetCreditBalance(account) と GetAccount(account) を取得
  5. 5分岐で creditPortion / stripePortion / paidVia / creditStopped を決定 (§5.3)
  6. creditPortion > 0 なら GrantCredit(amount = -creditPortion, type=consumed)
  7. ev に PaidVia / CreditAmountMinor / StripeAmountMinor をセット
  8. RecordUsageAccounting(ev, date) で usage_events / account_usage_daily / budget を atomic 更新
  9. stripePortion > 0 なら proratedTokens で按分して provider.ReportTokenUsage
 10. UsageRecordResult を返す
```

### 5.2 cost 計算

```go
cost_minor = (input_tokens  * input_cost_per_mtoken_minor)  / 1_000_000
           + (output_tokens * output_cost_per_mtoken_minor) / 1_000_000
```

整数除算で切り捨て。`/ 1_000_000` 未満の端数は無視。
pricing が見つからない (`ErrNotFound`) ときは `cost_minor = 0` で event は保存（forensic 用）。

### 5.3 5 分岐ルール（厳密版）

`balance = GetCreditBalance(account)`、`cost = cost_minor`、`isFree = account.Plan == "free"`

| ケース | 条件 | credit | stripe | paid_via | credit_stopped |
|---|---|---|---|---|---|
| 1 | `balance >= cost` | cost | 0 | `credit` | false |
| 2 | `0 < balance < cost && !isFree` | balance | cost - balance | `mixed` | false |
| 3 | `0 < balance < cost && isFree` | balance | 0 | `mixed` | **true** |
| 4 | `balance <= 0 && !isFree` | 0 | cost | `stripe` | false |
| 5 | `balance <= 0 && isFree` | 0 | 0 | `credit` | **true** |

不変条件:
- `credit + stripe == cost`（ケース 3, 5 を除く。停止扱いは accounting だけ走らせる）
- `paid_via == credit` のとき Stripe meter は **絶対に呼ばれない**（重複課金禁止）
- `credit_stopped == true` のとき worker は job を停止する責任を負う

### 5.4 Token 按分（`proratedTokens`）

Stripe meter には token 数を送るため、cost を split したぶん token も比例配分する:

```go
stripe_in  = ceil(input_tokens  * stripe_portion / total_cost)  // 切り上げ
stripe_out = floor(output_tokens * stripe_portion / total_cost) // 切り捨て
```

input 側を切り上げる理由: 極小 mixed event で `0 input token` を送らないため
（Stripe meter は value > 0 でないと no-op）。

### 5.5 冪等性

- `EventID` が同一の `RecordUsage` 呼び出しはすべて idempotent でなければならない。
  - `usage_events.event_id` は PRIMARY KEY、`ON CONFLICT DO NOTHING`。
  - クレジット消費 grant も `credit_id = "deduct-" + event_id` で衝突回避。
  - Stripe meter idempotency key は `event_id:in` / `event_id:out`。
- 再送は安全。ただし `account_usage_daily` の `event_count` は ON CONFLICT 上で
  `+1` されるため、**重複送信は count を増やす**。冪等にしたい場合は
  `InsertUsageEvent` の affected rows を見て daily update をスキップする要修正
  （現状未対応、将来改善候補）。

### 5.6 Budget Exceeded フラグ

- `accounts.budget_limit_minor > 0` のとき、`SumUsageCostByAccount` の累計が
  budget を超えたら `accounts.budget_exceeded = true` を立てる。
- `UsageRecordResult.BudgetExceeded = exceeded || credit_stopped`。
- worker はこのフラグが立った時点で **次の LLM コールを呼ばない**。
  処理中のものは完走させる。

---

## 6. 認可ルール

`api-authorization-contract.md` も併読。

### 6.1 ユーザー RPC

`authorizeAccount(ctx, accountID, actorUserID)`:

1. `accountID == ""` → `ErrInvalidArgument`
2. `actorUserID == ""` → `ErrUnauthenticated`
3. `IsAccountAccessible(accountID, actorUserID) == false` → `ErrNotFound`（存在を漏らさない）
4. `GetAccount(accountID)` を返す

### 6.2 Service-only RPC

`RecordUsage` は handler 層で `middleware.IsServiceCall(ctx)` をチェック。
`X-Synthify-Service-Token` ヘッダが `SYNTHIFY_INTERNAL_SERVICE_TOKEN` と一致しないと
即 `Unauthenticated`。service token は **環境変数経由で worker / api 間のみで共有**、
ユーザーには到達しない。

### 6.3 Webhook

`HandleWebhook` は HTTP body と `Stripe-Signature` ヘッダから
`provider.ParseWebhook` で署名検証。署名失敗は `ErrBillingWebhookSignatureInvalid` で
HTTP 400。本物の場合のみ `recordAndApplyWebhookEvent` に進む。

### 6.4 Admin RPC

`GrantCredit` は actor_user_id が admin である必要がある（実装は account_users role
に依存、TODO: より明示的な admin check）。`GrantFreeSignupCredit` は内部呼び出し専用で
RPC 露出していない（workspace 作成時に handler が直接呼ぶ）。

---

## 7. Stripe 連携詳細

### 7.1 Checkout flow

```
client -> CreateCheckoutSession(account, plan, currency)
  service.CreateCheckoutSession
    1. authorizeAccount
    2. plan/currency validate
    3. provider.EnsureCustomer -> stripe_customer_id を accounts に保存
    4. provider.CreateCheckoutSession -> URL を返す
  client は URL に redirect、Stripe Checkout で支払い情報入力
Stripe -> POST /api/v1/billing/webhook (checkout.session.completed)
  service.HandleWebhook
    1. ParseWebhook (署名検証)
    2. recordAndApplyWebhookEvent
       - billing_webhook_events に INSERT（冪等性キー: provider+event_id）
       - 既に処理済みなら skip
       - ApplyBillingEvent: accounts.plan / status / subscription を更新
       - MarkBillingWebhookEventProcessed
```

### 7.2 Meter Events

`config.Stripe` の `MeterInputEvent` / `MeterOutputEvent` で meter 名を指定。
`reportMeterEvent` は `value <= 0` または event 名が空なら no-op。

送信 payload:

```
event_name: <MeterInputEvent or MeterOutputEvent>
payload[stripe_customer_id]: <account.StripeCustomerID>
payload[value]: <input or output tokens>
identifier: <event_id>:in or <event_id>:out
timestamp: <unix>
```

### 7.3 Webhook イベント種別

| Stripe event | 反映先 |
|---|---|
| `checkout.session.completed` | plan / status 更新、stripe_customer_id 紐付け |
| `customer.subscription.updated` | plan / status / current_period_end |
| `customer.subscription.deleted` | plan = free, status = canceled |
| `invoice.paid` | invoices テーブル更新、status = active |
| `invoice.payment_failed` | status = past_due |

---

## 8. クレジット運用

### 8.1 サインアップ無料付与

`workspace.CreateWorkspace` の最後で `billing.GrantFreeSignupCredit(account.AccountID)`
を呼ぶ。冪等性は `credit_id = "free-signup-" + accountID` で担保
（重複 INSERT は `ON CONFLICT DO NOTHING`、mock 側も同等の重複チェック）。

付与額: `domain.FreeSignupCreditMinor = 100` ($1.00)。

### 8.2 残高計算

```sql
SELECT COALESCE(SUM(amount_minor), 0)::bigint FROM account_credits WHERE account_id = $1;
```

正の grant と負の `consumed` 行を合算するだけ。残高の表示は
`GetBillingAccount` レスポンスの `credit_balance` で 1 回取得する。

### 8.3 消費の記録

`RecordUsage` がクレジットを消費する際は **必ず以下の grant を 1 行 INSERT**:

```go
domain.CreditGrant{
    CreditID:    "deduct-" + event_id,   // 冪等性キー
    AccountID:   account_id,
    CreditType:  CreditTypeConsumed,
    AmountMinor: -credit_portion,        // 負
    Currency:    currency,
    Note:        "usage:" + event_id,
    GrantedBy:   "system",
    GrantedAt:   <RFC3339>,
}
```

### 8.4 管理者付与

`BillingHandler.GrantCredit`:

- actor が admin であることを確認（TODO: 厳密化）
- `credit_id = "manual-" + ulid()` で新規行を INSERT
- `note` は admin が指定したコメント
- 返値: 付与後の残高

---

## 9. DB スキーマ

### 9.1 `usage_events`

```sql
event_id            TEXT PRIMARY KEY,
account_id          TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
workspace_id        TEXT NOT NULL DEFAULT '',
job_id              TEXT NOT NULL DEFAULT '',
model               TEXT NOT NULL,
input_tokens        BIGINT NOT NULL DEFAULT 0,
output_tokens       BIGINT NOT NULL DEFAULT 0,
cost_minor          BIGINT NOT NULL DEFAULT 0,
currency            TEXT NOT NULL DEFAULT 'usd',
created_at          TIMESTAMPTZ NOT NULL,
paid_via            TEXT NOT NULL DEFAULT 'stripe',  -- 'credit' | 'stripe' | 'mixed'
credit_amount_minor BIGINT NOT NULL DEFAULT 0,
stripe_amount_minor BIGINT NOT NULL DEFAULT 0
```

Index:
- `idx_usage_events_account_created (account_id, created_at DESC)` — 期間集計
- `idx_usage_events_job (job_id) WHERE job_id <> ''` — job 単位の集計
- `idx_usage_events_paid_via (account_id, paid_via)` — 課金経路別の集計

### 9.2 `account_usage_daily`

日次ロールアップ。`(account_id, usage_date, model)` を PRIMARY KEY とし、
upsert で `input_tokens / output_tokens / cost_minor / event_count` を加算する。

### 9.3 `account_credits`

```sql
credit_id       TEXT PRIMARY KEY,
account_id      TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
credit_type     TEXT NOT NULL,         -- 'free' | 'purchased' | 'consumed'
amount_minor    BIGINT NOT NULL,       -- 正=付与, 負=消費
currency        TEXT NOT NULL DEFAULT 'usd',
note            TEXT NOT NULL DEFAULT '',
granted_by      TEXT NOT NULL DEFAULT '',
granted_at      TIMESTAMPTZ NOT NULL
```

Index: `idx_account_credits_account (account_id, granted_at DESC)`

### 9.4 `model_pricing`

```sql
model                        TEXT PRIMARY KEY,
input_cost_per_mtoken_minor  BIGINT NOT NULL,
output_cost_per_mtoken_minor BIGINT NOT NULL,
currency                     TEXT NOT NULL DEFAULT 'usd',
effective_from               TIMESTAMPTZ NOT NULL,
notes                        TEXT NOT NULL DEFAULT '',
display_multiplier           NUMERIC(6,2) NOT NULL DEFAULT 1.0
```

Seed: `db/init/100_seed.sql`。Gemini Flash (x1), Gemini Pro (x5), Gemini 2.0 Flash (x1)
など。

### 9.5 `accounts` (billing 関連列)

```sql
plan                  TEXT NOT NULL DEFAULT 'free',
billing_status        TEXT NOT NULL DEFAULT 'free',
stripe_customer_id    TEXT NOT NULL DEFAULT '',
stripe_subscription_id TEXT NOT NULL DEFAULT '',
budget_limit_minor    BIGINT NOT NULL DEFAULT 0,  -- 0 = 無制限
budget_exceeded       BOOL   NOT NULL DEFAULT FALSE,
current_period_end    TIMESTAMPTZ,
...
```

### 9.6 `billing_webhook_events`

冪等性ストア。`(provider, event_id)` を UNIQUE 制約とし、`recordAndApplyWebhookEvent`
が二重処理を防ぐ。`status` (`pending` / `processed` / `failed`) と `error_message` を持つ。

---

## 10. 設定 (`config.Billing`, `config.Stripe`)

`packages/shared/config/config.go`

```go
type Billing struct {
    FreeSignupCreditMinor int64  // ENV: BILLING_FREE_SIGNUP_CREDIT_MINOR (default 100)
}

type Stripe struct {
    SecretKey         string  // ENV: STRIPE_SECRET_KEY
    WebhookSecret     string  // ENV: STRIPE_WEBHOOK_SECRET
    SuccessURL        string  // ENV: STRIPE_SUCCESS_URL
    CancelURL         string  // ENV: STRIPE_CANCEL_URL
    PriceUSDMonthly   string  // ENV: STRIPE_PRICE_USD_MONTHLY (usage_based 紐付け)
    MeterInputEvent   string  // ENV: STRIPE_METER_INPUT_EVENT
    MeterOutputEvent  string  // ENV: STRIPE_METER_OUTPUT_EVENT
}
```

worker 側:

```go
type Worker struct {
    APIBaseURL          string  // ENV: SYNTHIFY_API_BASE_URL
    InternalServiceToken string // ENV: SYNTHIFY_INTERNAL_SERVICE_TOKEN
    ...
}
```

`InternalServiceToken` は **API 側の同名 ENV と必ず一致** させる。空のときは
worker の usage reporter は no-op になる（local dev 用の安全装置）。

---

## 11. ローカル/テスト挙動

| シーン | 振る舞い |
|---|---|
| `provider == nil` (Stripe 未設定) | Checkout / Portal / Meter は no-op、Webhook RPC は 503 |
| `usage == nil` (UsageRepository 未配線) | `RecordUsage` は info ログ + Stripe meter のみ |
| `mock.Store` | 全 repository を in-memory で実装。`SeedPricing`, `GrantCredit`, `UsageEvents()` 等のテスト helper あり |
| `accounts.StripeCustomerID == ""` | `ReportTokenUsage` は no-op（free プラン or Stripe 未登録） |

テストで `usage_based` プランを使うときは:

```go
account.Plan = string(domain.BillingPlanUsageBased)
account.StripeCustomerID = "cus_test"
```

の 2 行を直接書く（mock の `ApplyBillingPlan` を経由しなくてよい）。

---

## 12. 失敗時の方針

| 失敗 | 挙動 |
|---|---|
| `GetCreditBalance` エラー | warn ログ、balance=0 として続行（Stripe 側で課金） |
| `GrantCredit` (consumed) エラー | warn ログ、usage_events は通常通り保存。**料金は失われる**ので将来要 retry queue |
| `RecordUsageAccounting` エラー | error ログ + 上位に返す。RecordUsage 全体が失敗 |
| `ReportTokenUsage` エラー | warn ログ、内部 accounting は成功扱い（job は止めない）|
| Webhook 署名検証失敗 | HTTP 400、DB 変更なし |
| Webhook 二重配信 | `billing_webhook_events` の UNIQUE で吸収、副作用なし |

---

## 13. 関連ファイル一覧

### Service / Handler

- `apps/api/internal/service/billing.go` — usecase 本体
- `apps/api/internal/service/billing_test.go` — 5分岐の境界テスト含む
- `apps/api/internal/handler/billing.go` — Connect handler + webhook HTTP handler
- `apps/api/internal/handler/billing_test.go`

### Provider

- `apps/api/internal/infrastructure/stripe/provider.go` — Stripe REST 実装
- `apps/api/internal/infrastructure/stripe/*_test.go`

### Repository

- `packages/shared/repository/interfaces.go` — `UsageRepository`, `AccountRepository`
- `packages/shared/repository/postgres/billing.go` — postgres 実装
- `packages/shared/repository/mock/store.go` — mock 実装 + テスト helper

### Worker

- `apps/worker/pkg/worker/metering/reporter.go` — `connectReporter.RecordUsage`
- `apps/worker/pkg/worker/metering/reporter_test.go`

### Proto

- `contracts/connectrpc/synthify/tree/v1/billing.proto`
- 生成物: `packages/shared/gen/synthify/tree/v1/billing.pb.go`,
  `treev1connect/billing.connect.go`

### DB

- `db/init/004_billing.sql` — accounts billing 列、stripe webhook events
- `db/init/005_usage_billing.sql` — usage_events, account_usage_daily, model_pricing, invoices, payment_methods
- `db/init/008_credits.sql` — account_credits, model_pricing.display_multiplier
- `db/init/009_usage_events_split.sql` — usage_events に paid_via / credit_amount / stripe_amount
- `db/init/100_seed.sql` — pricing seed
- `db/queries/billing.sql` — sqlc クエリ

---

## 14. 未解決事項 / 将来改善

- `GrantCredit` の admin 判定が account_users.role に暗黙依存。明示的な admin claim を導入したい。
- `account_usage_daily.event_count` が重複送信で増える問題（§5.5）。
- 予算 90% で UI 通知 / degraded worker モード（[../improvements/usage-based-billing.md](../improvements/usage-based-billing.md) 末尾）。
- クレジット消費の grant 失敗時の再試行キュー。
- `purchased` クレジット（プリペイド購入）の Stripe Payment Intent 連携は未実装。
