# Invoice / Payment Method 同期設計

## 状態: ドラフト (Draft)
最終更新日: 2026-06-18

関連:

- 課金契約: [../contracts/billing-contract.md](../contracts/billing-contract.md)
- DB スキーマ: [../../db/migrations/0006_usage_billing.up.sql](../../db/migrations/0006_usage_billing.up.sql)
- Provider 実装: `apps/api/internal/infrastructure/stripe/provider.go`

---

## 0. 背景と問題

`invoices` / `payment_methods` テーブルは migration 0006 で全カラム・unique index
込みで定義済み。proto (`appv1.Invoice` / `appv1.PaymentMethod`)、フロント
(`InvoicePaper.tsx` / `ManagePaper.tsx`)、サービス層 (`ListInvoices` /
`ListPaymentMethods`) も配線済み。

**しかし両テーブルへの書き込み経路が一切存在しない。**

- `db/queries/billing.sql` の両テーブル参照は `ListInvoices` / `ListPaymentMethods`
  の SELECT のみ。INSERT/UPSERT/UPDATE はゼロ。
- sqlc 生成物にも `Upsert*` 関数なし。
- Webhook の `ApplyBillingEvent` は `UPDATE accounts ...` だけで invoice 明細・
  payment method を保存しない（`invoice.paid` を受けても明細は捨てている）。
- Provider が叩く Stripe API は customers / checkout / portal / subscriptions /
  meter_events の 5 つのみ。`/v1/invoices` も `/v1/payment_methods` も呼ばない。

結果として `ListInvoices` / `ListPaymentMethods` は **常に空を返す**。UI に枠は
あるが中身が永遠に出ない未実装機能。`billing.go:186` の "Stripe-synced cache"
というコメントは、同期処理が存在しないため誇大表現になっている。

## 1. 目標

1. `invoices` テーブルを Stripe の invoice ライフサイクルに追従させ、請求書一覧を
   実データで返す。
2. `payment_methods` テーブルを Stripe の支払い方法に追従させ、登録カード一覧と
   デフォルト判定を返す。
3. 既存の「自前DBキャッシュ + Webhook + reconcile」アーキテクチャに整合させる。
4. proto / フロントは変更しない（既存フィールドで充足）。

非目標（v1 スコープ外）:

- `InvoiceList.UpcomingAmount` / `UpcomingPeriodEnd` の Stripe 由来算出。当面は
  当期 usage からの概算 or `"0.00"` 据え置き。
- 請求書 PDF の自前再生成（Stripe の `hosted_invoice_url` / `invoice_pdf` を流用）。

## 2. データフロー（確定方針）

両リソースとも **Webhook push（Stripe → App）を主経路**とする。pull（都度フェッチ）
は採らない。理由はアーキテクチャ整合（accounts と同じく webhook で DB を更新し、
取りこぼしは reconcile で補う）と、表示のたびに Stripe を叩かない一貫性。

```
Stripe ──(invoice.*  / payment_method.* / customer.updated)──▶ Webhook handler
                                                                    │
                                              ParseWebhook で抽出     │
                                                                    ▼
                                              UpsertInvoice / UpsertPaymentMethod
                                                                    │
                                                                    ▼
                                              ListInvoices / ListPaymentMethods (SELECT)
```

過去分・取りこぼしは reconcile（App → Stripe の GET）でバックフィルする（§6）。

## 3. Invoices 設計

### 3.1 購読する Webhook イベント

| Stripe イベント | 反映 status | 備考 |
|---|---|---|
| `invoice.finalized` | `open` | hosted_invoice_url / invoice_pdf が確定 |
| `invoice.paid` / `invoice.payment_succeeded` | `paid` | paid_at を `status_transitions.paid_at` から |
| `invoice.payment_failed` | `open` (要再試行) | amount_due を保持 |
| `invoice.marked_uncollectible` | `uncollectible` | |
| `invoice.voided` | `void` | |

既存の `invoice.paid` / `invoice.payment_succeeded` / `invoice.payment_failed` は
すでに plan/status 抽出のため `ParseWebhook` で捌いている。これらに invoice 明細の
抽出を追加し、`finalized` / `marked_uncollectible` / `voided` を新規に追加する。

### 3.2 Stripe オブジェクトのパース拡張

`provider.go` の `stripeObject` に invoice 用フィールドを追加（現状は
customer/subscription/status/metadata しか拾っていない）:

```go
// stripeObject に追加
HostedInvoiceURL  string `json:"hosted_invoice_url"`
InvoicePDF        string `json:"invoice_pdf"`
AmountPaid        int64  `json:"amount_paid"`
AmountDue         int64  `json:"amount_due"`
Total             int64  `json:"total"`
Created           int64  `json:"created"`
StatusTransitions struct {
    PaidAt int64 `json:"paid_at"`
} `json:"status_transitions"`
// currency は既存の stripePrice ではなく invoice 直下にも存在するため
// stripeObject.Currency string `json:"currency"` を追加
// period_start / period_end は lines.data[0].period.{start,end}
```

`stripeLineItem` に `Period struct{ Start, End int64 }` を追加して期間を取得。

### 3.3 ドメイン型

`ProviderWebhookEvent` に invoice ペイロードを添付（`invoice.*` のときだけ非 nil）:

```go
type ProviderInvoice struct {
    StripeInvoiceID  string
    AmountMinor      int64  // paid: amount_paid / それ以外: total or amount_due
    Currency         string
    Status           string // paid | open | uncollectible | void
    HostedInvoiceURL string
    InvoicePDFURL    string
    PeriodStart      string // RFC3339
    PeriodEnd        string
    PaidAt           string // RFC3339, 未払いは ""
    CreatedAt        string
}

// ProviderWebhookEvent に追加
Invoice *ProviderInvoice `json:"invoice,omitempty"`
```

`ParseWebhook` の invoice 系 case で `providerEventFromPrice(...)` の戻り値に
`ev.Invoice = p.invoiceFromObject(obj)` を設定する新ヘルパを追加。

### 3.4 書き込みクエリ

`invoices` の PK は `invoice_id`（自前）、`stripe_invoice_id` に unique index。
**`invoice_id = stripe_invoice_id`** とし、PK 衝突 = 同一請求書として冪等 upsert。

```sql
-- name: UpsertInvoice :exec
INSERT INTO invoices (
  invoice_id, account_id, stripe_invoice_id, amount_minor, currency, status,
  hosted_invoice_url, invoice_pdf_url, period_start, period_end, paid_at,
  created_at, updated_at
) VALUES (
  $1, $2, $1, $3, $4, $5, $6, $7,
  sqlc.narg('period_start')::timestamptz,
  sqlc.narg('period_end')::timestamptz,
  sqlc.narg('paid_at')::timestamptz,
  $8, $8
)
ON CONFLICT (invoice_id) DO UPDATE SET
  amount_minor       = EXCLUDED.amount_minor,
  status             = EXCLUDED.status,
  hosted_invoice_url = EXCLUDED.hosted_invoice_url,
  invoice_pdf_url    = EXCLUDED.invoice_pdf_url,
  period_start       = EXCLUDED.period_start,
  period_end         = EXCLUDED.period_end,
  paid_at            = EXCLUDED.paid_at,
  updated_at         = EXCLUDED.updated_at;
```

`$1 = stripe_invoice_id`, `$2 = account_id`, `$8 = updated/created at`。
時刻は RFC3339 文字列 → `time.Time` でパースして渡す（NULL 可は narg）。

### 3.5 サービス配線

`billingService.recordAndApplyWebhookEvent`（`billing.go:321`）の
`ApplyBillingEvent` 成功後に追記:

```go
if event.Invoice != nil && s.usage != nil {
    if err := s.usage.UpsertInvoice(ctx, event.AccountID, event.Invoice); err != nil {
        s.logger.Error("billing.webhook.invoice_upsert_failed",
            "error", err.Error(),
            "account_id", event.AccountID,
            "stripe_invoice_id", event.Invoice.StripeInvoiceID,
        )
        // accounts 更新は成功済み。webhook 自体は ack（再送に任せず冪等 upsert で吸収）
    }
}
```

`RecordBillingWebhookEvent` の冪等チェックが既にあり、かつ upsert なので二重適用は安全。

### 3.6 account_id の解決

invoice オブジェクトの `metadata.account_id` は subscription_data 由来で乗ることが
多いが、無い場合は `ExternalCustomerID` から逆引きが必要。既存の `ApplyBillingEvent`
が customer→account の解決を持つため、同じ経路を `UpsertInvoice` でも使う
（`event.AccountID` が空なら customer から引く repo メソッドを利用）。

## 4. Payment Methods 設計（Webhook push）

### 4.1 購読する Webhook イベント

| Stripe イベント | 操作 |
|---|---|
| `payment_method.attached` | upsert（brand/last4/exp） |
| `payment_method.automatically_updated` | upsert（カード更新時の exp など） |
| `payment_method.detached` | 削除 |
| `customer.updated` | `invoice_settings.default_payment_method` で is_default 再計算 |

### 4.2 デフォルト追跡の難所

`is_default` は単一 PM の属性ではなく顧客の
`invoice_settings.default_payment_method` で決まる。`customer.updated` 受信時に
**そのアカウントの全 PM の is_default を一旦 false → 該当1件を true** に更新する
必要がある。`payment_method.attached` だけでは default は分からない。

```sql
-- name: UpsertPaymentMethod :exec
INSERT INTO payment_methods (
  payment_method_id, account_id, stripe_payment_method_id,
  brand, last4, exp_month, exp_year, is_default, created_at, updated_at
) VALUES ($1, $2, $1, $3, $4, $5, $6, $7, $8, $8)
ON CONFLICT (payment_method_id) DO UPDATE SET
  brand      = EXCLUDED.brand,
  last4      = EXCLUDED.last4,
  exp_month  = EXCLUDED.exp_month,
  exp_year   = EXCLUDED.exp_year,
  updated_at = EXCLUDED.updated_at;

-- name: DeletePaymentMethod :exec
DELETE FROM payment_methods WHERE stripe_payment_method_id = $1;

-- name: SetDefaultPaymentMethod :exec
UPDATE payment_methods
SET is_default = (stripe_payment_method_id = $2),
    updated_at = $3
WHERE account_id = $1;
```

`payment_method_id = stripe_payment_method_id` を採用（invoice と同方針）。

### 4.3 Stripe オブジェクトのパース

`payment_method.*` の object は `card.{brand,last4,exp_month,exp_year}` を持つ。
`stripeObject` に:

```go
Card struct {
    Brand    string `json:"brand"`
    Last4    string `json:"last4"`
    ExpMonth int64  `json:"exp_month"`
    ExpYear  int64  `json:"exp_year"`
} `json:"card"`
InvoiceSettings struct {
    DefaultPaymentMethod string `json:"default_payment_method"`
} `json:"invoice_settings"` // customer.updated 用
```

### 4.4 ドメイン型と配線

`ProviderWebhookEvent` に `PaymentMethod *ProviderPaymentMethod` と
`PaymentMethodDeleted string`（detach 時の stripe id）、
`DefaultPaymentMethod string`（customer.updated 時）を追加。サービス層で
イベント種別に応じて Upsert / Delete / SetDefault を呼び分ける。

## 5. Webhook ハンドラの拡張点まとめ

`ParseWebhook` の switch に case を追加:

```go
case "invoice.finalized", "invoice.marked_uncollectible", "invoice.voided":
    ev := p.providerEventFromInvoice(event, obj) // plan は触らず Invoice のみ
    return ev, nil
case "payment_method.attached", "payment_method.automatically_updated":
    return p.providerEventFromPaymentMethod(event, obj, false), nil
case "payment_method.detached":
    return p.providerEventFromPaymentMethod(event, obj, true), nil
// customer.updated は default PM 再計算のため新規 case
case "customer.updated":
    return p.providerEventFromCustomerDefault(event, obj), nil
```

注意: invoice/PM 専用イベントでは `event.Plan` を空のままにする。
`recordAndApplyWebhookEvent` の `event.Plan == "" && Status != CheckoutPending`
で "ignored" マークされる既存分岐に当たらないよう、**Invoice / PaymentMethod
ペイロードが非 nil なら "ignored" 早期 return をスキップ**する分岐修正が必要
（§3.5 の追記より前に判定順を調整）。

## 6. バックフィル（reconcile 拡張）

この機能リリース以前の請求は webhook が来ないため、`reconcileAccount`
（`billing.go:408`）に invoice 取得を追加:

```go
// GET /v1/invoices?customer=X&limit=N → 各 invoice を UpsertInvoice
// GET /v1/payment_methods?customer=X&type=card → UpsertPaymentMethod
//   + customer.invoice_settings.default_payment_method で SetDefault
```

既存 reconcile CLI 経路（`ReconcileAccount` / `ReconcileLinkedAccounts`）にそのまま
乗る。冪等 upsert なので何度走らせても安全。pull はこの保守経路に限定し、通常の
表示パスでは使わない。

## 7. 影響ファイル

| ファイル | 変更 |
|---|---|
| `provider.go` | stripeObject 拡張、invoice/PM/customer 抽出ヘルパ、ParseWebhook の case 追加、reconcile 用 GET |
| `domain/billing.go` | `ProviderInvoice` / `ProviderPaymentMethod` 追加、`ProviderWebhookEvent` にフィールド追加 |
| `db/queries/billing.sql` | `UpsertInvoice` / `UpsertPaymentMethod` / `DeletePaymentMethod` / `SetDefaultPaymentMethod` |
| `repository/postgres/billing.go`, `interfaces.go` | 上記 4 メソッドの実装とインターフェイス追加 |
| `repository/mock/store.go` | モック追加 |
| `service/billing.go` | webhook 適用後の Upsert 呼び分け、ignored 分岐の判定順調整、reconcile バックフィル |
| sqlc 生成物 | **v1.31.1 で再生成**（ローカル v1.27.0 で生成すると全ファイル diff が出る既知事象） |
| proto / フロント | **変更なし** |

## 8. テスト方針

- `provider_test.go`: 各 invoice/PM/customer イベントの fixture JSON →
  `ParseWebhook` が期待する `ProviderInvoice` / `ProviderPaymentMethod` を返すか。
  署名検証は既存ヘルパを流用。
- `billing_test.go`: `HandleWebhook` が Upsert/Delete/SetDefault を正しい引数で
  呼ぶか（モック検証）。ignored 分岐に誤って落ちないこと。
- `*_sql_test.go`: Upsert の冪等性（同 stripe id 2 回 → 1 行）、
  SetDefault が全件 false→1件 true にすること。
- 重複 webhook（同 event_id 再送）で 1 回しか適用されないこと。

## 9. 段階リリース案

1. **PR1**: invoices 書き込み経路（§3）。リスク低・単独で価値。
2. **PR2**: payment_methods 書き込み経路（§4）。default 追跡を含む。
3. **PR3**: reconcile バックフィル（§6）。過去分の充填。

各 PR は前段に依存せず独立してマージ可能。
