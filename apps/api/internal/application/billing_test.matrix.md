# テストマトリクス: `billing_test.go`

このマトリクスは、`billing_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

> **読み方の注意**: 下の「テストケース表」は *既存テストが何を確認しているか* を写したものなので、
> **テストが1件も無いメソッド／分岐はこの表に現れない**。取りこぼしを防ぐため、
> 「インターフェース網羅チェック」「依存エラー軸」「未テスト分岐 (GAP)」の各節を併読すること。
> カバレッジ数値は `go test -coverprofile ./apps/api/internal/application -run Billing` の実測 (2026-06-11)。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

## インターフェース網羅チェック

`BillingUsecase` (12) + `BillingReconciler` (2) の全メソッドに対し、専用テストの有無と実測カバレッジを突き合わせる。
**テスト0件＝下のテストケース表に1行も現れないので、ここで検出する。**

coverage は 2026-06-12 にギャップ閉じテスト22件を追加した後の実測。

| メソッド | 専用テスト | coverage | 状態 | 残りの未テスト分岐 |
| --- | --- | --- | --- | --- |
| `GetBillingAccount` | ✅ 2件 (06-12) | 100% | OK | — |
| `CreateCheckoutSession` | ✅ 6件 | 88.5% | OK | provider_failed (provider が error) の直接経路。 |
| `CreatePortalSession` | ✅ 4件 | 84.2% | OK | customer_failed (ensureCustomer error)。 |
| `HandleWebhook` | ✅ 11件 | 100% | OK | — (apply_failed の repo error は依存エラー軸へ) |
| `GetUsage` | ✅ 3件 | 83.3% | PARTIAL | parseMinor 失敗行の黙殺 (:531)、usage repo error。 |
| `RecordUsage` | ✅ 8件 | 87.0% | OK | PricingMissing→NewRelic notice、recorder!=nil で generic error。 |
| `UpdateBudget` | ✅ 4件 | 86.7% | OK | usage repo の UpdateAccountBudgetLimit error。 |
| `ListInvoices` | ✅ 2件 | 83.3% | PARTIAL | usage==nil スタブ、非空 mapping、repo error。 |
| `ListPaymentMethods` | ✅ 2件 | 83.3% | PARTIAL | usage==nil で nil 返し、非空 mapping、repo error。 |
| `GrantFreeSignupCredit` | ✅ 2件 (06-12) | 75.0% | PARTIAL | 冪等性 (credit_id衝突) は **mock が dedup しないため DB 統合テスト送り**、GrantCredit失敗 warn。 |
| `GrantCredit` | ✅ 3件 (06-12) | 87.5% | OK | usage repo の GrantCredit error。 |
| `GetCreditBalance` | ✅ 2件 (06-12) | 80.0% | OK | usage==nil 返り (間接)、repo error。 |
| `ReconcileAccount` | ✅ 5件 | 83.3% | OK | GetAccount error (repo)。 |
| `ReconcileLinkedAccounts` | ✅ 1件 | 71.4% | PARTIAL | 途中 account の失敗で partial diffs+error、ListStripeLinkedAccounts error。 |

補助関数の実測: `formatMinor` **100%** / `parseMinor` **96.7%** / `newCreditID` **100%** /
`reconcileAccount` 90.9% / `recordAndApplyWebhookEvent` 65.4% (invalid currency / "ignored" mark が残り) /
`shouldNoticeBillingError` 42.9% / `noticeError` 55.6% / `ensureProviderCustomer` 55.6%。

**2026-06-12 追加分 (22件)**: `GetBillingAccount` x2 / `GrantCredit` x3 / `GetCreditBalance` x2 /
`GrantFreeSignupCredit` x2 / checkout customer_failed / portal provider==nil・provider_failed /
webhook provider==nil・parse_failed・invalid plan / reconcile provider==nil・fetch失敗・apply収束 /
GetUsage usage==nil / UpdateBudget invalid・JPY / `formatMinor`/`parseMinor` 通貨ラウンドトリップ。

**残る主GAP**: repo (accounts/usage) のエラー伝播は **mock store にフォールト注入フックが無い**ため未着手。
failing decorator か mock 拡張が前提（下記「依存エラー軸」参照）。`recordAndApplyWebhookEvent` の
invalid currency / "ignored" mark、`GetUsage:531` の不正行黙殺もまだ。

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestCreateCheckoutSession_InvalidPlan_WarnsAndReturnsError` | `CreateCheckoutSession` | plan validation | invalid plan で checkout session を作る。 | error を返し warn log を出す。 | provider を呼ばない。 | error, warn log | 不正 plan の拒否。 | plan 名の大小文字、空 plan。 | empty plan / deprecated plan。 | PARTIAL |
| [ ] | `TestCreateCheckoutSession_OtherUser_DeniedAndReturnsNotFound` | `CreateCheckoutSession` | 認可 | 他 user の account で checkout を作る。 | NotFound 系で拒否する。 | session を作らない。 | error | account 所有者境界。 | 存在しない account との優先順位。 | unknown account と owner mismatch の比較。 | PARTIAL |
| [ ] | `TestCreateCheckoutSession_InvalidCurrency_WarnsAndReturnsError` | `CreateCheckoutSession` | currency validation | invalid currency を指定する。 | error を返し warn log を出す。 | provider を呼ばない。 | error, warn log | currency allowlist。 | 大文字小文字、空 currency default。 | supported currency の table-driven test。 | PARTIAL |
| [ ] | `TestCreateCheckoutSession_ProviderNotConfigured_LogsError` | `CreateCheckoutSession` | provider config | provider 未設定で checkout を作る。 | error を返し error log を出す。 | session を作らない。 | error, error log | Stripe provider 未設定時の failure。 | provider timeout/error の詳細 mapping。 | provider error 別 log level。 | PARTIAL |
| [ ] | `TestCreateCheckoutSession_Success_LogsInfo` | `CreateCheckoutSession` | 正常系 | valid plan/currency/provider で checkout を作る。 | session URL を返し info log を出す。 | provider が呼ばれる。 | success, info log | checkout session 成功経路。 | provider request payload 全体。 | success URL/cancel URL/customer ID の検証。 | PARTIAL |
| [ ] | `TestCreatePortalSession_OtherUser_DeniedAndReturnsNotFound` | `CreatePortalSession` | 認可 | 他 user の account で portal を作る。 | NotFound 系で拒否する。 | provider を呼ばない。 | error | portal の account 所有者境界。 | customer ID 欠損。 | no Stripe customer の error。 | PARTIAL |
| [ ] | `TestCreatePortalSession_Success_UsesAuthorizedAccount` | `CreatePortalSession` | 正常系 | authorized account で portal を作る。 | portal session を返す。 | provider が authorized account で呼ばれる。 | success | portal session 成功経路。 | return URL validation。 | provider payload の詳細。 | PARTIAL |
| [ ] | `TestHandleWebhook_InvalidSignature_Warns` | `HandleWebhook` | webhook signature | invalid signature の webhook を処理する。 | error を返し warn log を出す。 | DB を変更しない。 | error, warn log | 署名検証失敗の拒否。 | raw body 欠落、replay。 | DB no-op の明示確認。 | PARTIAL |
| [ ] | `TestHandleWebhook_Success_LogsInfo` | `HandleWebhook` | webhook 正常系 | valid webhook を処理する。 | 成功し info log を出す。 | event を保存/処理する。 | no error, info log | webhook 処理成功の基本経路。 | event type 別詳細。 | processed event の永続化 assert。 | PARTIAL |
| [ ] | `TestHandleWebhook_CheckoutCompletedMarksPending` | `HandleWebhook` | checkout completed | checkout completed event を処理する。 | subscription pending 状態に反映する。 | account/subscription 状態が更新される。 | state assertions | checkout completed の pending 反映。 | metadata 欠落、不正 customer。 | missing account metadata。 | PARTIAL |
| [ ] | `TestHandleWebhook_DuplicateEventNoop` | `HandleWebhook` | idempotency | 同じ webhook event を2回処理する。 | 2回目は no-op。 | 二重 mutation しない。 | duplicate no-op | webhook event idempotency。 | 同時 duplicate、partial failure 後再実行。 | concurrent duplicate event。 | PARTIAL |
| [ ] | `TestHandleWebhook_OrderingInvoiceAndSubscriptionConverges` | `HandleWebhook` | event ordering | invoice/subscription event を順不同で処理する。 | 最終状態が収束する。 | entitlement/subscription 状態が整合する。 | final state | webhook ordering 耐性。 | 他 event type の順序。 | deleted/failed と invoice paid の順序。 | PARTIAL |
| [ ] | `TestHandleWebhook_PaymentFailureKeepsEntitlementAndMarksPastDue` | `HandleWebhook` | payment failure | payment failure event を処理する。 | entitlement は維持しつつ past due にする。 | account billing state が更新される。 | state assertions | 支払い失敗時の猶予挙動。 | repeated failure、grace period。 | failure count / grace period。 | PARTIAL |
| [ ] | `TestHandleWebhook_SubscriptionDeletedReturnsFree` | `HandleWebhook` | subscription deletion | subscription deleted event を処理する。 | free plan に戻る。 | entitlement が free に更新される。 | state assertions | subscription 終了時の downgrade。 | 未払い/キャンセル直後の境界。 | deleted 後 invoice paid が来た場合。 | PARTIAL |
| [ ] | `TestHandleWebhook_UnknownPriceIDIgnored` | `HandleWebhook` | unknown price | unknown price ID の event を処理する。 | 破壊的変更をせず ignore する。 | entitlement を不正更新しない。 | no harmful mutation | unknown Stripe price の安全な無視。 | alert/log の扱い。 | unknown price の warn log。 | PARTIAL |
| [ ] | `TestReconcileAccount_DryRunDoesNotMutate` | `ReconcileAccount` | dry-run | dry-run で account reconcile する。 | 差分は報告するが mutation しない。 | DB 変更なし。 | dry-run result, no mutation | reconcile dry-run safety。 | 複数 subscription。 | no mutation の全 field assert。 | PARTIAL |
| [ ] | `TestReconcileAccount_ApplyMutates` | `ReconcileAccount` | apply | apply で account reconcile する。 | Stripe state に合わせて mutation する。 | account billing state が更新される。 | state assertions | reconcile apply の反映。 | provider error、unknown price。 | provider failure 時の no partial mutation。 | PARTIAL |
| [ ] | `TestReconcileLinkedAccountsListsStripeCustomers` | `ReconcileLinkedAccounts` | linked accounts | linked account の Stripe customer を list する。 | 対象 customers が返る/処理される。 | 必要に応じて reconcile される。 | list assertions | linked account reconciliation の入口。 | pagination、大量 customer。 | Stripe customer pagination。 | PARTIAL |
| [ ] | `TestGetUsage_OtherUser_DeniedAndReturnsNotFound` | `GetUsage` | 認可 | 他 user の usage を読む。 | NotFound 系で拒否する。 | 状態変化なし。 | error | usage read の account 所有者境界。 | admin/system access。 | service account 経由 read。 | PARTIAL |
| [ ] | `TestGetUsage_Success_ReturnsUsageReport` | `GetUsage` | 正常系 | usage event/rollup を持つ account の usage を読む。 | usage report を返す。 | 状態変化なし。 | report assertions | usage reporting 成功経路。 | date range、timezone。 | period boundary の usage aggregation。 | PARTIAL |
| [ ] | `TestRecordUsage_MissingFields_ReturnsUsageEventInvalid` | `RecordUsage` | validation | 必須 field 欠落の usage event を記録する。 | `UsageEventInvalid` 系 error を返す。 | event を保存しない。 | error | usage event 必須 field validation。 | field ごとの個別 error。 | table-driven missing fields。 | PARTIAL |
| [ ] | `TestRecordUsage_ComputesCostFromPricing_PersistsEventAndRollup` | `RecordUsage` | cost accounting | pricing ありの model/token usage を記録する。 | cost を計算し、event と rollup を保存する。 | usage event / rollup / account state が更新される。 | cost, persisted event, rollup | usage accounting の基本経路。 | 丸め境界、複数通貨。 | decimal rounding boundary。 | PARTIAL |
| [ ] | `TestRecordUsage_UnknownModel_CostZeroButStillPersisted` | `RecordUsage` | unknown model | pricing 不明 model の usage を記録する。 | cost 0 で event は保存する。 | usage event が残る。 | cost zero, persisted | unknown model でも監査 event を残すこと。 | alert/log、後から pricing 追加。 | unknown model warning。 | PARTIAL |
| [ ] | `TestRecordUsage_TogglesBudgetExceededOnFirstCross` | `RecordUsage` | budget threshold | usage 記録で budget を初回超過する。 | budget exceeded が true になる。 | account budget state が変わる。 | budget flag | budget crossing detection。 | すでに exceeded、閾値ちょうど。 | equal threshold / already exceeded。 | PARTIAL |
| [ ] | `TestRecordUsage_NilUsageRepo_FallsBackToLoggingStub` | `RecordUsage` | fallback | usage repo nil で記録する。 | panic せず logging stub に fallback する。 | 永続化なし。 | no panic/error behavior | usage repo 未設定時の互換 fallback。 | production での設定漏れ検知。 | nil repo warning log。 | PARTIAL |
| [ ] | `TestRecordUsage_StripeMeterFailureWarnsButKeepsAccounting` | `RecordUsage` | external failure | Stripe meter 送信が失敗する。 | warn するが accounting は保存する。 | local usage accounting は進む。 | warn, persisted accounting | external meter failure と local accounting の分離。 | retry queue、再送。 | meter failure retry record。 | PARTIAL |
| [ ] | `TestRecordUsage_CreditCoversFullCost_NoStripeMeter` | `RecordUsage` | credit | credit が全額を覆う usage を記録する。 | Stripe meter は送らず credit を消費する。 | credit balance / usage が更新される。 | no stripe meter, credit assertions | credit 全額充当。 | credit ちょうど、複数 credit grant。 | credit exact boundary。 | PARTIAL |
| [ ] | `TestRecordUsage_MixedPaymentSplitsTokensProportionally` | `RecordUsage` | credit + paid split | credit と paid が混在する usage を記録する。 | tokens/cost が比例按分される。 | credit と paid usage が分割記録される。 | split assertions | mixed payment の按分。 | 端数丸め、非常に小さい cost。 | proportional split rounding。 | PARTIAL |
| [ ] | `TestRecordUsage_FreePlanWithoutCredit_StopsWithoutStripe` | `RecordUsage` | free plan gate | free plan で credit なしの usage を記録する。 | Stripe 送信せず停止/拒否する。 | paid accounting を進めない。 | no stripe, stop assertions | free plan の課金停止。 | budget flag、error type。 | stopped usage の audit event 有無。 | PARTIAL |
| [ ] | `TestUpdateBudget_OtherUser_DeniedAndReturnsNotFound` | `UpdateBudget` | 認可 | 他 user の budget を更新する。 | NotFound 系で拒否する。 | budget を変えない。 | error | budget update の account 所有者境界。 | admin/system update。 | failed update 後の値不変。 | PARTIAL |
| [ ] | `TestUpdateBudget_Success_PersistsLimit` | `UpdateBudget` | 正常系 | authorized user が budget limit を更新する。 | limit が保存される。 | account budget limit が更新される。 | persisted limit | budget limit update。 | 負数、0、currency。 | invalid budget limit。 | PARTIAL |
| [ ] | `TestListInvoices_OtherUser_DeniedAndReturnsNotFound` | `ListInvoices` | 認可 | 他 user の invoices を list する。 | NotFound 系で拒否する。 | provider を呼ばない。 | error | invoice list の account 境界。 | customer ID 欠損。 | provider not configured。 | PARTIAL |
| [ ] | `TestListInvoices_Success_ReturnsEmptyList` | `ListInvoices` | 正常系 | authorized user が invoice list を取得する。 | 空 list を返す。 | 状態変化なし。 | empty list | invoice list 成功経路。 | invoice がある場合、pagination。 | non-empty invoice mapping。 | PARTIAL |
| [ ] | `TestListPaymentMethods_OtherUser_DeniedAndReturnsNotFound` | `ListPaymentMethods` | 認可 | 他 user の payment methods を list する。 | NotFound 系で拒否する。 | provider を呼ばない。 | error | payment method list の account 境界。 | customer ID 欠損。 | provider not configured。 | PARTIAL |
| [ ] | `TestListPaymentMethods_Success_ReturnsEmptyList` | `ListPaymentMethods` | 正常系 | authorized user が payment methods を取得する。 | 空 list を返す。 | 状態変化なし。 | empty list | payment method list 成功経路。 | card がある場合、pagination。 | non-empty payment method mapping。 | PARTIAL |

## 観点別チェックグリッド

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

| テストケース | 正常系 | 異常系 | 認可 | validation | webhook | idempotency/order | accounting | credit | budget | Stripe provider | logging | 永続化副作用 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestCreateCheckoutSession_InvalidPlan_WarnsAndReturnsError` | - | ☑ | - | ☑ | - | - | - | - | - | ◐ | ☑ | ◐ |
| `TestCreateCheckoutSession_OtherUser_DeniedAndReturnsNotFound` | - | ☑ | ☑ | - | - | - | - | - | - | - | ◐ | ◐ |
| `TestCreateCheckoutSession_InvalidCurrency_WarnsAndReturnsError` | - | ☑ | - | ☑ | - | - | - | - | - | ◐ | ☑ | ◐ |
| `TestCreateCheckoutSession_ProviderNotConfigured_LogsError` | - | ☑ | - | - | - | - | - | - | - | ☑ | ☑ | ◐ |
| `TestCreateCheckoutSession_Success_LogsInfo` | ☑ | - | ☑ | ☑ | - | - | - | - | - | ☑ | ☑ | ◐ |
| `TestCreatePortalSession_OtherUser_DeniedAndReturnsNotFound` | - | ☑ | ☑ | - | - | - | - | - | - | - | ◐ | - |
| `TestCreatePortalSession_Success_UsesAuthorizedAccount` | ☑ | - | ☑ | - | - | - | - | - | - | ☑ | ◐ | - |
| `TestHandleWebhook_InvalidSignature_Warns` | - | ☑ | - | ☑ | ☑ | - | - | - | - | ☑ | ☑ | ◐ |
| `TestHandleWebhook_Success_LogsInfo` | ☑ | - | - | - | ☑ | ◐ | - | - | - | ☑ | ☑ | ☑ |
| `TestHandleWebhook_CheckoutCompletedMarksPending` | ☑ | - | - | - | ☑ | ◐ | - | - | - | ☑ | ◐ | ☑ |
| `TestHandleWebhook_DuplicateEventNoop` | ☑ | - | - | - | ☑ | ☑ | - | - | - | ☑ | ◐ | ☑ |
| `TestHandleWebhook_OrderingInvoiceAndSubscriptionConverges` | ☑ | - | - | - | ☑ | ☑ | - | - | - | ☑ | ◐ | ☑ |
| `TestHandleWebhook_PaymentFailureKeepsEntitlementAndMarksPastDue` | ☑ | - | - | - | ☑ | ◐ | - | - | - | ☑ | ◐ | ☑ |
| `TestHandleWebhook_SubscriptionDeletedReturnsFree` | ☑ | - | - | - | ☑ | ◐ | - | - | - | ☑ | ◐ | ☑ |
| `TestHandleWebhook_UnknownPriceIDIgnored` | - | ☑ | - | ☑ | ☑ | ◐ | - | - | - | ☑ | ◐ | ☑ |
| `TestReconcileAccount_DryRunDoesNotMutate` | ☑ | - | ◐ | - | - | - | - | - | - | ☑ | ◐ | ☑ |
| `TestReconcileAccount_ApplyMutates` | ☑ | - | ◐ | - | - | - | - | - | - | ☑ | ◐ | ☑ |
| `TestReconcileLinkedAccountsListsStripeCustomers` | ☑ | - | ◐ | - | - | - | - | - | - | ☑ | ◐ | ◐ |
| `TestGetUsage_OtherUser_DeniedAndReturnsNotFound` | - | ☑ | ☑ | - | - | - | ◐ | - | - | - | - | - |
| `TestGetUsage_Success_ReturnsUsageReport` | ☑ | - | ☑ | - | - | - | ☑ | - | - | - | - | - |
| `TestRecordUsage_MissingFields_ReturnsUsageEventInvalid` | - | ☑ | - | ☑ | - | - | ☑ | - | - | - | - | ◐ |
| `TestRecordUsage_ComputesCostFromPricing_PersistsEventAndRollup` | ☑ | - | - | ☑ | - | - | ☑ | - | ◐ | - | - | ☑ |
| `TestRecordUsage_UnknownModel_CostZeroButStillPersisted` | ☑ | - | - | ☑ | - | - | ☑ | - | - | - | ◐ | ☑ |
| `TestRecordUsage_TogglesBudgetExceededOnFirstCross` | ☑ | - | - | - | - | - | ☑ | - | ☑ | - | - | ☑ |
| `TestRecordUsage_NilUsageRepo_FallsBackToLoggingStub` | ☑ | - | - | - | - | - | ◐ | - | - | - | ☑ | - |
| `TestRecordUsage_StripeMeterFailureWarnsButKeepsAccounting` | ☑ | ☑ | - | - | - | - | ☑ | - | - | ☑ | ☑ | ☑ |
| `TestRecordUsage_CreditCoversFullCost_NoStripeMeter` | ☑ | - | - | - | - | - | ☑ | ☑ | - | ☑ | - | ☑ |
| `TestRecordUsage_MixedPaymentSplitsTokensProportionally` | ☑ | - | - | - | - | - | ☑ | ☑ | - | ☑ | - | ☑ |
| `TestRecordUsage_FreePlanWithoutCredit_StopsWithoutStripe` | - | ☑ | - | - | - | - | ☑ | ☑ | ◐ | ☑ | ◐ | ☑ |
| `TestUpdateBudget_OtherUser_DeniedAndReturnsNotFound` | - | ☑ | ☑ | - | - | - | - | - | ☑ | - | - | ◐ |
| `TestUpdateBudget_Success_PersistsLimit` | ☑ | - | ☑ | ◐ | - | - | - | - | ☑ | - | - | ☑ |
| `TestListInvoices_OtherUser_DeniedAndReturnsNotFound` | - | ☑ | ☑ | - | - | - | - | - | - | ◐ | - | - |
| `TestListInvoices_Success_ReturnsEmptyList` | ☑ | - | ☑ | - | - | - | - | - | - | ☑ | - | - |
| `TestListPaymentMethods_OtherUser_DeniedAndReturnsNotFound` | - | ☑ | ☑ | - | - | - | - | - | - | ◐ | - | - |
| `TestListPaymentMethods_Success_ReturnsEmptyList` | ☑ | - | ☑ | - | - | - | - | - | - | ☑ | - | - |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| checkout / portal payload | success/error/logging は確認済み。 | Stripe provider に渡す success URL、cancel URL、return URL、customer ID の詳細。 |
| webhook | signature、duplicate、ordering、主要 subscription lifecycle は確認済み。 | concurrent duplicate、partial failure retry、deleted 後 invoice paid など矛盾 event。 |
| reconcile | dry-run と apply は確認済み。 | provider error 時の no partial mutation、pagination。 |
| usage accounting | cost 計算、unknown model、budget crossing、credit split は確認済み。 | decimal rounding、threshold ちょうど、very small cost、複数 credit grant。 |
| Stripe meter | failure でも local accounting 継続、credit 時 no meter は確認済み。 | retry queue / retry record、meter payload の詳細。 |
| listing | empty list success は確認済み。 | non-empty invoice/payment method mapping、pagination。 |
| credits | (なし) — `GrantFreeSignupCredit` は setup でしか呼ばれない。 | `GrantCredit` admin 付与全般、`GetCreditBalance` の認可、free signup の冪等性と金額。 |
| account read | (なし) | `GetBillingAccount` の認可成功/拒否。 |
| currency 整形 | usd 経路のみ。 | `formatMinor`/`parseMinor` の JPY (小数なし)、負数、小数3桁以上→BudgetInvalid。 |

## 未テスト分岐 (GAP) — テストケース表に現れない経路

メソッド単位ではなく分岐単位の穴。カバレッジ HTML の赤行に対応。

| 場所 (billing.go) | 分岐 | 期待挙動 | なぜ重要か |
| --- | --- | --- | --- |
| `GetBillingAccount` :106 | 全体 | 認可 → account 返却 / 他ユーザは NotFound | メソッドごと未テスト (0%)。 |
| `GrantCredit` :801 | `amountMinor<=0` / `usage==nil` / 成功 | BudgetInvalid / ProviderNotConfigured / grant 返却 | admin 付与経路がまるごと未テスト (0%)。 |
| `GetCreditBalance` :824 | 認可失敗 / usage==nil / 成功 | 0+err / 0 / balance | service の認可ラッパ未通過。 |
| `GrantFreeSignupCredit` :772 | 2回目呼び出し | credit_id 衝突で冪等 no-op | 二重付与＝収益漏れ。setup 流用では assert していない。 |
| `HandleWebhook` :250 | `provider==nil` | ProviderNotConfigured + Error log | 設定漏れ検知。 |
| `HandleWebhook` :269 | 署名以外の parse 失敗 | parse_failed Error log + NewRelic notice | 署名失敗 (warn) と別経路。 |
| `recordAndApplyWebhookEvent` :342-353 | event の plan/currency invalid | `MarkProcessed(..,"failed",..)` | 不正イベントの failed 永続化。 |
| `recordAndApplyWebhookEvent` :322 | `event==nil` | (false,nil) no-op | nil 安全性。 |
| `ensureProviderCustomer` :314 | 新 customer ID 返却 | `SetAccountStripeCustomerID` で永続化 | customer ID の取り違え防止。44% のみ。 |
| `CreateCheckout/Portal` :161,:219 | `ensureProviderCustomer` がエラー | customer_failed Error log + return | provider 障害時の握り潰し防止。 |
| `reconcileAccount` :424 | apply=true かつ既に一致 | `ApplyBillingEvent` を呼ばない no-op | 無駄 mutation 防止。apply テストは差分有りのみ。 |
| `ReconcileLinkedAccounts` :398 | 途中 account の失敗 | partial diffs + error 返却 | 一括処理の部分失敗挙動。 |
| `GetUsage` :531 | `parseMinor` が行で失敗 | `continue` で黙って除外 | **不正コスト行が total から無言で消える** — 実害バグ候補。要 assert。 |
| `GetUsage` :511 | `usage==nil` | ゼロ report スタブ | 未配線時の挙動。 |
| `UpdateBudget` :726 | parseMinor 失敗 / 負数 | BudgetInvalid | 入力検証。usage==nil で非永続(:729)も。 |
| `RecordUsage` :587 | `PricingMissing` | NewRelic notice | UnknownModel(cost 0) とは別の課金漏れアラート。 |
| `reportStripeMeterPortion` :642 | `GetAccount` 失敗/nil | meter 送らず無言 return | meter 欠落の silent skip。 |
| `formatMinor`/`parseMinor` :657,:673 | JPY (小数なし) / 負数 / frac>2 | 整数整形 / 符号 / BudgetInvalid | 通貨別ロジックが単体で一度も通っていない。 |
| `shouldNoticeBillingError` :481 | 各 case (plan/currency/signature/NotFound) | NewRelic 抑制 vs notice | 28.6% のみ。誤アラート/見逃しの分類。 |

## 依存エラー軸 (dependency returns error)

各メソッドが外部 (`provider` / `accounts` / `usage`) のエラーをどう伝播するか。
☑=テスト有 / ◐=間接 / ❌=未テスト。

| メソッド | provider err | accounts(repo) err | usage(repo) err |
| --- | --- | --- | --- |
| `CreateCheckoutSession` | ☑ provider_failed | ❌ EnsureCustomer/SetID 失敗 | - |
| `CreatePortalSession` | ❌ provider_failed | ❌ customer_failed | - |
| `HandleWebhook` | ☑ signature / ❌ parse_failed | ❌ Record/Apply/Mark 失敗 | - |
| `ReconcileAccount` | ❌ FetchBillingState 失敗 | ❌ GetAccount / ApplyBillingEvent 失敗 | - |
| `GetUsage` | - | ◐ 認可 | ❌ ListUsageByModel/ListDailyUsage 失敗 |
| `RecordUsage` | ☑ meter 失敗 | ❌ GetAccount 失敗 | ◐ recorder error |
| `UpdateBudget` | - | ◐ 認可 | ❌ UpdateAccountBudgetLimit 失敗 |
| `ListInvoices` | - | ◐ 認可 | ❌ ListInvoices 失敗 |
| `ListPaymentMethods` | - | ◐ 認可 | ❌ ListPaymentMethods 失敗 |
| `GrantCredit` | - | - | ❌ GrantCredit 失敗 |

→ repo/provider のエラー伝播はほぼ全面的に未テスト。mock に `err` を返させる軽い table-driven test で一気に埋められる。

## 境界値チェックマトリクス

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 数値 / count / size

| 対象値 / 条件 | `0` | `1` | typical | `max - 1` | `max` | `max + 1` | negative | huge / overflow | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| usage token count | - | - | ☑ | - | - | - | - | - | `TestRecordUsage_ComputesCostFromPricing_PersistsEventAndRollup`, `TestRecordUsage_MixedPaymentSplitsTokensProportionally` | 0 token、1 token、巨大 token、負数。 |
| usage cost / rounding | - | - | ☑ | - | - | - | - | - | `TestRecordUsage_ComputesCostFromPricing_PersistsEventAndRollup`, `TestRecordUsage_MixedPaymentSplitsTokensProportionally` | very small cost、round half、複数通貨。 |
| credit balance | ☑ | ◐ | ☑ | - | - | - | - | - | `TestRecordUsage_CreditCoversFullCost_NoStripeMeter`, `TestRecordUsage_MixedPaymentSplitsTokensProportionally`, `TestRecordUsage_FreePlanWithoutCredit_StopsWithoutStripe` | credit ちょうど、複数 credit grant、期限切れ credit。 |
| budget threshold | - | - | ☑ | ◐ | ☑ | ☑ | - | - | `TestRecordUsage_TogglesBudgetExceededOnFirstCross`, `TestUpdateBudget_Success_PersistsLimit` | threshold ちょうど、already exceeded、budget 0/negative。 |
| webhook duplicate count | ☑ | ☑ | - | - | - | - | - | - | `TestHandleWebhook_DuplicateEventNoop` | concurrent duplicate、partial failure 後の再実行。 |
| list result count | ☑ | - | - | - | - | - | - | - | `TestListInvoices_Success_ReturnsEmptyList`, `TestListPaymentMethods_Success_ReturnsEmptyList` | 1件、複数件、pagination、provider error。 |

### 文字列 / ID

| 対象値 / 条件 | empty | whitespace | valid existing | valid missing | malformed | other scope | deleted / inaccessible | max length + 1 / huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| accountID / owner user | - | - | ☑ | ◐ | - | ☑ | - | - | `TestCreateCheckoutSession_OtherUser_DeniedAndReturnsNotFound`, `TestGetUsage_OtherUser_DeniedAndReturnsNotFound`, `TestUpdateBudget_OtherUser_DeniedAndReturnsNotFound` | unknown account、empty accountID、admin/system caller。 |
| Stripe customer ID | - | - | ☑ | - | - | - | - | - | `TestCreatePortalSession_Success_UsesAuthorizedAccount`, `TestReconcileLinkedAccountsListsStripeCustomers` | empty customer ID、deleted customer、malformed customer ID。 |
| Stripe price ID | - | - | ☑ | - | ☑ | - | - | - | `TestHandleWebhook_UnknownPriceIDIgnored`, `TestHandleWebhook_SubscriptionDeletedReturnsFree` | empty price ID、複数 item subscription。 |

### enum / role / status

| 対象値 / 条件 | empty / default | allowed value | disallowed value | unknown value | transition before | transition after | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| checkout plan | - | ☑ | ☑ | ☑ | - | - | `TestCreateCheckoutSession_InvalidPlan_WarnsAndReturnsError`, `TestCreateCheckoutSession_Success_LogsInfo` | empty plan、deprecated plan、大小文字違い。 |
| checkout currency | ◐ | ☑ | ☑ | ☑ | - | - | `TestCreateCheckoutSession_InvalidCurrency_WarnsAndReturnsError`, `TestCreateCheckoutSession_Success_LogsInfo` | empty currency default、大小文字違い、全 supported currency。 |
| webhook event type | - | ☑ | - | ◐ | ☑ | ☑ | `TestHandleWebhook_OrderingInvoiceAndSubscriptionConverges`, `TestHandleWebhook_SubscriptionDeletedReturnsFree` | unknown event type、deleted 後 invoice paid。 |
| reconcile mode | - | ☑ | - | - | ☑ | ☑ | `TestReconcileAccount_DryRunDoesNotMutate`, `TestReconcileAccount_ApplyMutates` | provider error 時の dry-run/apply 差、pagination。 |
| Stripe meter state | - | ☑ | - | - | ☑ | ☑ | `TestRecordUsage_StripeMeterFailureWarnsButKeepsAccounting`, `TestRecordUsage_CreditCoversFullCost_NoStripeMeter` | retry queue、meter payload validation。 |

### webhook / 外部依存

| 対象値 / 条件 | not configured | success | invalid response | returns error | called once | not called | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Stripe provider | ☑ | ☑ | - | ☑ | ☑ | ☑ | `TestCreateCheckoutSession_ProviderNotConfigured_LogsError`, `TestCreateCheckoutSession_Success_LogsInfo`, `TestRecordUsage_StripeMeterFailureWarnsButKeepsAccounting` | provider timeout、typed errors、payload 詳細。 |
| webhook signature | - | ☑ | ☑ | ☑ | - | - | `TestHandleWebhook_InvalidSignature_Warns`, `TestHandleWebhook_Success_LogsInfo` | empty signature、replay timestamp、raw body mismatch。 |
