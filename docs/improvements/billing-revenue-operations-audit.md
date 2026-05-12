# Billing / Revenue Operations Audit

調査日: 2026-05-12

この文書は Stripe 導入を「checkout を作れる」状態で止めず、課金・利用制限・税務・運用事故まで含めて安全に本番投入するための改善メモである。

## 1. Current Code Findings

現状の課金実装は、Stripe 連携の入口はできつつあるが、課金の正本として使うにはまだ不足がある。

- `accounts` は `plan`, `storage_quota_bytes`, `storage_used_bytes`, `max_file_size_bytes`, `stripe_customer_id`, `stripe_subscription_id` を持つ。
- `documents.file_size` と `document_files.file_size` は存在する。
- frontend は `File.size` を `CreateDocument` に渡し、backend はその申告値を `documents.file_size` に保存している。
- 実アップロード後に GCS object metadata の `size` を確認していない。
- `accounts.storage_used_bytes` を document 作成・削除・失敗時に増減していない。
- `storage_quota_bytes` / `max_file_size_bytes` を upload URL 発行または document 作成時に強制していない。
- `Workspace` proto には plan/quota/usage 用 field があるが、mapper は返していない。
- webhook event の永続化・重複排除がない。
- Stripe API の create 系 POST に app 側の idempotency key を付けていない。
- success URL 到達と DB 反映が分離されておらず、UI 上の pending / reflected 状態設計が未確定。

結論: 現状の `file_size` は「表示・記録用の申告値」であり、課金・quota 判定の信頼できる値ではない。

## 2. External Billing Assumptions

### Stripe Checkout

Stripe Checkout は Checkout Session を server 側で作成し、その URL に redirect する hosted checkout として使う。Stripe は「支払い成功後の session は Customer と active Subscription などを参照する」と説明しているため、app 側は success URL だけで権限解放せず、webhook または Stripe object の確認で subscription 状態を反映する。

Relevant docs:
- https://docs.stripe.com/api/checkout/sessions
- https://docs.stripe.com/payments/checkout

### Customer Portal

Customer Portal は subscription / payment method / invoice / customer details を Stripe-hosted UI で管理するためのもの。portal session は短命なので、ユーザーが管理画面を開くたびに on-demand で作る。

Portal は便利だが、usage-based billing や複数 product など一部 subscription は更新不可で cancel のみになる制限がある。将来従量課金に寄せる場合、portal での変更可能範囲を先に確認する必要がある。

Relevant docs:
- https://docs.stripe.com/api/customer_portal/sessions
- https://docs.stripe.com/billing/subscriptions/customer-portal

### Webhooks

webhook は raw body と `Stripe-Signature` を使って検証する。JSON parse 後の body では署名検証できない場合があるため、HTTP handler は body をそのまま provider に渡す。

Stripe subscription flow では、初回支払い後の access provision は `invoice.paid` または subscription status の確認を使うのが基本。`checkout.session.completed` は checkout 完了を示すが、非同期決済や subscription lifecycle の全ケースを単独で表すものではない。

Relevant docs:
- https://docs.stripe.com/webhooks/signature
- https://docs.stripe.com/billing/subscriptions/webhooks
- https://docs.stripe.com/billing/subscriptions/overview

### Idempotency

Stripe の POST request は idempotency key を受け取る。network retry で Customer / Checkout Session / Portal Session を二重作成しないため、app 側で安定した key を付ける。

Relevant docs:
- https://docs.stripe.com/api/idempotent_requests

### Tax / Invoices / PCI

Checkout は hosted payment page として使えるため、card data を app 側に通さない設計にできる。ただし PCI compliance は shared responsibility であり、Stripe を使えば完全に不要になるわけではない。

Tax は Stripe Tax / automatic tax を使う選択肢がある。Japan / global SaaS で invoices や tax ID 表示が必要になる場合、Customer Portal で tax IDs を管理できる。

Relevant docs:
- https://docs.stripe.com/security/guide
- https://docs.stripe.com/payments/checkout/automatic_taxes
- https://docs.stripe.com/billing/customer/tax-ids
- https://docs.stripe.com/tax

### JPY / USD Currency Support

Stripe Price は複数通貨を持てる。Stripe docs では multi-currency Price を作成し、`currency_options[jpy][unit_amount]` のように通貨ごとの金額を設定できると説明されている。Checkout は Price が顧客の local currency をサポートしていればその通貨を使い、未対応なら default currency を使う。

ただし Adaptive Pricing は `subscription` mode の Checkout Session には適用されない。SaaS subscription で JPY / USD を明示的にサポートするなら、Adaptive Pricing ではなく multi-currency Price、または通貨別 Price ID を app 側で選ぶ設計にする。

JPY は zero-decimal currency であり、Stripe API の `amount` / `unit_amount` はそのまま円単位で渡す。USD は minor unit が cent なので `10 USD` は `1000` として渡す。app DB でも `amount_minor` と `currency` を分けて保持し、金額計算に float を使わない。

Relevant docs:
- https://docs.stripe.com/products-prices/manage-prices
- https://docs.stripe.com/products-prices/how-products-and-prices-work
- https://docs.stripe.com/currencies
- https://docs.stripe.com/payments/checkout/present-local-currencies

## 3. Required Source Of Truth

### Payment Source Of Truth

Stripe が決済・請求の正本。

App DB は authorization / entitlement / quota enforcement の正本。app の機能解放は必ず app DB の billing state を参照する。

禁止:
- success URL に戻っただけで `pro` 扱いにすること。
- frontend から送られた plan / price / quota 値を信頼すること。
- Stripe object ID なしに有料権限を永続化すること。

### Usage Source Of Truth

`accounts.storage_used_bytes` は backend が管理する。frontend の `File.size` は事前見積もりとしてのみ使う。

信頼できる size は以下のどちらか:
- upload 完了後に GCS object metadata から取得した size
- backend 経由 upload の場合、backend が stream しながら count した byte 数

現行の signed URL 直 PUT 構成なら、GCS metadata 確認が必要。

## 4. Data Model Gaps

追加または明文化が必要な永続化:

### billing_events

Stripe webhook の重複処理を防ぐ。

Minimum columns:
- `provider`
- `event_id`
- `event_type`
- `received_at`
- `processed_at`
- `processing_status`
- `account_id`
- `stripe_customer_id`
- `stripe_subscription_id`
- unique `(provider, event_id)`

重複排除は `(provider, event_id)` の unique constraint で行う。`payload_hash` は不要（Stripe では webhook signature 検証が改ざん検知を担うため）。

### account billing state

`accounts` には現状の Stripe ID だけでなく、subscription lifecycle を表す field が欲しい。

候補:
- `billing_status`: `free`, `checkout_pending`, `active`, `past_due`, `unpaid`, `canceled`, `incomplete`
- `stripe_price_id`
- `billing_currency`: `jpy` or `usd`
- `billing_amount_minor`
- `billing_interval`: `month` or `year`
- `current_period_end`
- `cancel_at_period_end`
- `billing_updated_at`

`plan` は product entitlement の単純化表現として残す。詳細 lifecycle は `billing_status` に寄せる。

### price catalog

JPY / USD を安全に扱うため、plan から Stripe Price ID への対応を env var だけに閉じ込めすぎない。初期は env でもよいが、少なくとも app 内部の catalog として次を持つ。

- `plan`
- `currency`
- `interval`
- `stripe_price_id`
- `amount_minor`
- `display_name`
- `active`

選択肢:

- 1つの multi-currency Stripe Price ID を使い、Checkout に任せる。
- JPY / USD それぞれの Stripe Price ID を持ち、app が `currency` を明示して session を作る。

初期方針は「通貨別 Price ID」を推奨する。理由は、subscription state と invoice 表示を app 側で説明しやすく、JPY / USD の価格差を明示的に管理できるため。

### upload reservations

直 PUT で quota を厳密に守るには、予約と確定が必要。

候補:
- `upload_reservations`
- `reservation_id`
- `account_id`
- `workspace_id`
- `document_id`
- `expected_size_bytes`
- `actual_size_bytes`
- `status`: `reserved`, `uploaded`, `confirmed`, `expired`, `failed`
- `expires_at`

予約時点で `storage_used_bytes + reserved_bytes <= storage_quota_bytes` を満たすことを保証する。

**Concurrent reservation の serialization:**

複数リクエストが同時に quota check → reserve を行う場合、以下のいずれかで排他制御する。

- `SELECT ... FOR UPDATE` で account row を lock し、check と reserve をトランザクション内でアトミックに行う（単一 DB、シンプルな実装向け）
- 楽観的ロック: `accounts` に `storage_version` (integer) を持ち、`UPDATE ... WHERE storage_version = $old` が 0 rows なら retry（競合が稀な場合に有効）

初期実装は `SELECT FOR UPDATE` を推奨。lock contention が問題になった場合に楽観的ロックへ移行する。

## 5. Upload / Quota Enforcement

### Required Checks

`GetUploadURL` or `CreateDocument` の前に account を取得し、以下を backend で判定する。

- `file_size > 0`
- `file_size <= account.max_file_size_bytes`
- `account.storage_used_bytes + active_reserved_bytes + file_size <= account.storage_quota_bytes`
- workspace が account に属し、actor が member であること

### Upload Completion

signed URL で upload したあと、processing 開始前に backend が object metadata を確認する。

- object が存在すること
- actual size が request の expected size と一致または許容範囲内であること
- mime type / extension policy に反していないこと
- actual size で usage を確定すること

`StartProcessing` は upload confirmed でない document を処理しない。

**GCS actual size 確認のタイミングと実装方式:**

初期実装はクライアント通知方式を推奨する。

1. クライアントが GCS への PUT 完了後、`ConfirmUpload` endpoint を叩く
2. backend が GCS object metadata (`storage.objects.get`) で `size` / `contentType` を取得
3. 検証 OK なら reservation を `confirmed` に遷移し、`storage_used_bytes` を actual size で確定する
4. 検証 NG なら GCS object を delete し、reservation を `failed` にして quota を解放する

将来的に GCS `object.finalize` Pub/Sub notification を使う方式も選択できるが、配信遅延・再配信の考慮が必要になるため初期は不採用。

### Failure / Delete / Reprocess

- upload 期限切れ reservation は定期 cleanup で解放する。
- document delete が入る場合、確定済み size を `storage_used_bytes` から差し引く。
- reprocess は新規 storage 使用量を増やさない。
- derived artifacts を storage に入れるなら、それも別 ledger でカウントするか、quota 対象外と明記する。

## 6. Stripe Event Handling Policy

初期導入で処理すべき event:

- `checkout.session.completed`
- `invoice.paid`
- `invoice.payment_failed`
- `customer.subscription.created`
- `customer.subscription.updated`
- `customer.subscription.deleted`
- `customer.tax_id.updated` if tax ID state is displayed in app

Policy:
- `invoice.paid` or active subscription confirmationで `pro` を provision する。
- `checkout.session.completed` は pending 解消の補助情報として扱う。
- `invoice.payment_failed` だけで即時 downgrade しない。`past_due` / grace period を表示する。
- `unpaid` は access revocation 候補。Stripe docs 上も product access を取り消すべき状態として扱われる。
- `customer.subscription.deleted` は `free` へ戻す。ただし period end cancel の場合は `customer.subscription.updated` で `cancel_at_period_end` を表示する。
- `customer.subscription.updated` では `items[].price.id` の変化を検知して `plan` / `stripe_price_id` を更新する。price_id → plan のマッピングは server config（env var または DB table）で管理し、unknown price_id は警告ログを出して state mutation しない。
- 未知 event は 2xx で受け、監査ログには残す。処理対象 event のみ state mutation する。

**Event 到着順について:**

Stripe は event の到着順を保証しない。`subscription.updated` が `invoice.paid` より先に届くケースと後に届くケースを両方考慮する。subscription status と invoice status の両方を確認して provision/revocation を判断する実装にすること（どちらか一方のみに依存しない）。

## 7. Checkout / Portal API Policy

Checkout Session:
- server side でのみ作成する。
- `account_id` を metadata と subscription metadata の両方に入れる。
- `customer` は必ず既存 Stripe Customer ID を渡す。
- `price_id` は env / server config / price catalog のみから決める。
- frontend から受け取ってよいのは `currency` の希望値まで。`price_id` や金額は受け取らない。
- 対応通貨は初期導入では `jpy` / `usd` のみ。
- request currency が未指定の場合、日本 locale / billing country なら `jpy`、それ以外は `usd` にする。判定できない場合の default は business decision として固定する。
- create request には idempotency key を付ける。
- `success_url` は「反映待ち」画面にする。

Portal Session:
- account member 以上、できれば owner/admin のみに制限する。
- portal session は都度作成する。
- portal で変更可能な product / price / cancellation 設定は Dashboard 側と app 側の plan model を同期する。

## 8. UI Requirements

Billing UI は次を表示できる必要がある。

- current plan
- billing status
- billing currency
- plan amount in the selected currency
- storage used / quota
- max file size
- checkout pending
- past due / payment failed
- cancel at period end
- manage billing button

成功 URL 直後は「決済反映待ち」にする。Stripe Checkout Session の有効期限は 24 時間であるため、`checkout_pending` 状態の最大保持時間も 24 時間を目安とする。それを超えても webhook が届かない場合は reconciliation job が検知し、`free` に戻すか support 導線を表示する。polling timeout は UX 上は 60〜120 秒程度を目安にする。

Upload UI は frontend だけで size 制限してはいけない。frontend validation は UX 用、backend validation が正本。

## 9. Operational Requirements

### Reconciliation Job

webhook missed / processing failure に備え、定期的に Stripe subscriptions と local account state を照合する。

Minimum:
- `stripe_customer_id` がある account を列挙
- Stripe subscription の latest status / price / period を取得
- local `plan` / `billing_status` / `stripe_subscription_id` と差分を記録
- 自動修正するか、まず dry-run log にする

### Observability

ログ・メトリクス:
- checkout session created / failed
- portal session created / failed
- webhook received / duplicate / applied / failed
- quota rejected
- upload reservation created / confirmed / expired
- reconciliation diff

ログに出してはいけないもの:
- Stripe secret key
- webhook secret
- raw card / payment method details
- full webhook payload unless redacted and access controlled

### Testing

必要なテスト:
- webhook signature validation
- duplicate event no-op（同一 event_id を 2 回処理しないこと）
- event ordering: `subscription.updated` → `invoice.paid` の順と `invoice.paid` → `subscription.updated` の順、両方で結果が同一になること
- payment failure -> grace / no immediate destructive downgrade
- subscription deleted -> free
- quota boundary: exact limit, over limit, concurrent uploads（reservation の race condition を含む）
- GCS actual size mismatch（expected と actual が異なる場合 reservation が解放されること）
- expired upload reservation cleanup
- `subscription.updated` で unknown price_id を受けた場合に state mutation しないこと

Stripe Test Clock を使うと subscription renewal / dunning / cancellation の時間経過を検証しやすい。

## 10. Suggested Implementation Phases

### Phase 1: Usage Guardrails

- `AccountRepository` に quota check / usage reservation / confirmation API を追加。
- `GetUploadURL` を quota-aware にする。
- upload completion confirmation endpoint (`ConfirmUpload`) を追加。
- `StartProcessing` は confirmed upload のみ許可。
- `Workspace` mapper で Phase 1 時点で利用可能な情報（`plan`, `storage_quota_bytes`, `storage_used_bytes`, `max_file_size_bytes`）を返す。`billing_status` / `current_period_end` は Phase 2 完了後に追加する。

### Phase 2: Billing State Machine

- `billing_events` を追加。
- `billing_status` など subscription lifecycle columns を追加。
- webhook 処理を idempotent にする。
- `invoice.paid`, `subscription.updated`, `subscription.deleted` を正しく反映。

### Phase 3: Stripe API Hardening

- Stripe create POST に idempotency key を付ける。
- JPY / USD の price catalog を追加する。
- checkout request に `currency` を追加し、server side で許可通貨・Price ID を解決する。
- Checkout に automatic tax を採用するか決める。
- Customer に email / locale / tax fields を入れるか決める。
- portal owner/admin 制限を実装。

### Phase 4: Ops / Reconciliation

- periodic reconciliation job を追加。
- billing admin / audit view を追加。
- test clock scenario を CI or manual runbook 化。

## 11. Open Decisions

- Free / Pro の quota 値をコード定数に置くか DB table にするか。
- upload storage に derived artifacts を含めるか。
- `invoice.payment_failed` 後の grace period を何日にするか。
- `past_due` 中に upload / processing を止めるか、既存 document 閲覧だけ許すか。
- Japan-only launch か global launch か。Stripe Tax / tax ID collection の要否が変わる。**Phase 3 開始前に決定必須**（`automatic_tax` の設定変更は Checkout Session の作り直しを伴うため、後から変えると手戻りが大きい）。
- default currency を `jpy` にするか `usd` にするか。
- JPY / USD を同一 multi-currency Price で扱うか、通貨別 Price ID で扱うか。
- 既存 subscription の通貨変更を許可するか。許可する場合、portal で変更できるか、cancel/resubscribe にするか。
- Customer Portal で cancellation を即時にするか period end にするか。
- 複数 workspace を持つ account で owner/admin 権限をどう billing 操作に対応させるか。
- price_id → plan のマッピングを env var で管理するか DB table で管理するか（plan が増えた場合の運用性に影響する）。
- concurrent reservation の serialization を `SELECT FOR UPDATE` で行うか楽観的ロックで行うか（Section 4 を参照）。

## 12. Non-Goals For Initial Release

- app 内で payment method 管理 UI を作ること。
- app 内で invoice PDF を再生成すること。
- usage-based billing への即時移行。
- frontend から任意 price ID を選ばせること。
- workspace 単位の課金。
