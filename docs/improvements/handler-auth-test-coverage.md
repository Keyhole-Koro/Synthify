# Handler auth test coverage

## Context

API handlers rely on `currentUser`, `authorizeWorkspace`, `authorizeDocument`, and `authorizeItem` to protect Connect RPC endpoints. Representative handler-level tests now cover workspace, document, tree, item, job, and billing session flows, but a few endpoints still need explicit coverage or authorization design work.

## Status

- ✅ Handler-level unauthenticated rejection tests added for all non-public RPCs (see `apps/api/internal/handler/auth_unauthenticated_test.go`).
- ✅ `BillingHandler` の全 RPC (`GetBillingAccount`, `GetUsage`, `UpdateBudget`, `ListInvoices`, `ListPaymentMethods`, `RecordUsage`) に handler テスト追加。
- ✅ Document side-effect endpoints (`GetUploadURL`, `StartProcessing`, `ResumeProcessing`) の未認証拒否テスト追加。
- ✅ `ApproveAlias` / `RejectAlias` の未認証拒否テスト追加。
- `RecordItemView` / `GetUserItemActivity` は Firestore 移行済みの no-op stub だったため、ItemService から削除済み。
- ✅ `ListAllJobs` に admin 認可を実装 (`middleware.IsAdmin` + `SYNTHIFY_ADMIN_USER_EMAILS` allowlist)。`AnonymousReadAllowed` で log-viewer は引き続き通過可能。
- ✅ `RecordUsage` を `middleware.IsServiceCall` (X-Synthify-Service-Token) 必須にし、内部 worker -> API 呼び出し以外を拒否。
- ✅ `authorizeDocument` / `authorizeItem` / `authorizeAndLoadJob` で `currentUser` を先にチェックし、未認証ユーザーへの情報漏洩 (NotFound 経由) を防止。

## Remaining work

- log endpoint (`ListJobLogs`, `SearchJobLogs`, `ListRelatedJobLogs`) の **anonymous-read context** での挙動テスト。現状は middleware が `AnonymousReadAllowed` を立てるパス指定だが、これを stable contract として handler テストでも検証する。
- `SYNTHIFY_INTERNAL_SERVICE_TOKEN` 未設定環境での `RecordUsage` の挙動 (現状は service call フラグが立たないので全拒否) を CI / dev environment でどう扱うか。worker と API のローカル統合テストを別途整える。
- Long-term: service token は環境変数ベースの暫定実装。本番では mTLS or short-lived signed tokens (e.g. GCP IAP, Cloud Run service-to-service auth) に移行する。

## Acceptance criteria

- Every non-public Connect RPC has at least one handler-level unauthenticated rejection test.
- Every resource-scoped handler has at least one other-user rejection test.
- Any intentionally public or anonymous endpoint is documented and tested as an explicit exception.
