# Security Hardening Audit

2026-05-24 の軽量監査メモ。優先度は「実害の近さ」と「設定ミス時の blast radius」で並べる。

## P1

- `DocumentService.GetUploadURL` が workspace 認可、upload reservation、quota accounting を迂回して signed upload URL を発行できる。通常アップロードは `CreateDocument` に一本化し、この legacy RPC は閉じる。
- Firestore rules の `/workspaces/{workspaceId}/jobs/{jobId}` read が `request.auth != null` のみで、workspace membership を見ていない。job status を API 経由に寄せるか、rules 側に membership 判定を追加する。

## P2

- CORS で `CORS_ALLOWED_ORIGINS="*"` を許すと、任意 Origin を反射しつつ `Access-Control-Allow-Credentials: true` を返す。設定時に reject するか credentials を落とす。
- Terraform の deployer default roles が強い。bootstrap 用権限と通常 deploy 用権限を分け、`resourcemanager.projectIamAdmin` / `secretmanager.admin` / `iam.serviceAccountTokenCreator` の常時付与を避ける。

## P3

- fake/dev GCS URL builder が workspace ID / object name を URL escape していない。本番 signed URL には直接影響しないが、dev/test の事故防止として escape する。
