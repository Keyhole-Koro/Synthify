# テストマトリクス: `billing_test.go`

このマトリクスは、`billing_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

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
