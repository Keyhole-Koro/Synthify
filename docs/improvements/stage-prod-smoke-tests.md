# Stage / Prod Smoke Test 導入計画

## 背景

Stage / Prod への deploy 後に、API・Worker・Frontend が最低限生きていることを自動確認する仕組みがない。

特に `/health` が落ちている、Cloud Run URL が解決できない、Firebase Hosting が配信できていない、といった重大劣化は deploy 直後に検知したい。一方で prod ではデータ作成・更新・削除を伴う E2E を走らせず、ユーザー影響のない smoke test に限定する。

## 方針

- Stage は deploy 後に API / Worker の `/health` を確認する。
- Prod は deploy 後に API / Worker の `/health` と Frontend のトップページ到達を確認する。
- `/health` は公開 liveness とし、`/health?ready=1` は deploy ごとの一時 API key で保護する。
- Stage のみ、後続で認証付き read API や代表的な Connect RPC の軽量チェックを追加する。
- Prod では destructive / write 系テストを禁止し、死活確認と副作用のない synthetic monitoring に限定する。

## 実装計画

### 1. Smoke test script

`scripts/smoke-test.sh` を追加し、対象ごとに必要な URL を環境変数で受け取る。

```sh
API_BASE_URL=https://... READINESS_API_KEY=... ./scripts/smoke-test.sh api
WORKER_BASE_URL=https://... READINESS_API_KEY=... ./scripts/smoke-test.sh worker
FRONTEND_URL=https://... ./scripts/smoke-test.sh frontend
```

初期実装では以下を確認する。

- `api`: `GET ${API_BASE_URL}/health` が 2xx を返す。
- `api`: `GET ${API_BASE_URL}/health?ready=1` が `X-Synthify-Readiness-Key` 付きで DB query を含めて 2xx を返す。
- `api`: 認証なしの Connect RPC が 401 を返し、auth gate が有効である。
- `worker`: `GET ${WORKER_BASE_URL}/health` が 2xx を返す。
- `worker`: `GET ${WORKER_BASE_URL}/health?ready=1` が `X-Synthify-Readiness-Key` 付きで DB query を含めて 2xx を返す。
- `worker`: 副作用のない invalid Connect request が 400 を返し、Connect route が生きている。
- `frontend`: `GET ${FRONTEND_URL}` が 2xx を返し、HTML app shell らしい本文を返す。

タイムアウト、DNS failure、非 2xx は非 0 exit とし、GitHub Actions を fail させる。

### 2. Backend deploy workflow

`.github/workflows/deploy-backend.yml` の Terraform apply 完了後に smoke test step を追加する。

- `terraform output -raw api_uri` を `API_BASE_URL` に渡す。
- GitHub Actions が deploy ごとに `READINESS_API_KEY` を生成し、Terraform var と smoke script の両方に渡す。
- Worker は internal ingress のため、`gcloud run services proxy synthify-worker-${ENVIRONMENT}` でローカル proxy を張り、`WORKER_BASE_URL=http://127.0.0.1:18081` を渡す。
- stage / prod の両方で API / Worker health を確認する。

### 3. Frontend deploy workflow

`.github/workflows/deploy-frontend.yml` の Firebase Hosting deploy 後に frontend smoke test step を追加する。

- `FRONTEND_URL` は GitHub Environment variable で受け取る。
- 未設定の場合は workflow を fail させる。
- prod でもトップページの 2xx 到達確認のみ行う。

## 将来拡張

- Stage で Firebase ID token を取得し、認証付き read API を確認する。
- Stage で代表的な Connect RPC に対して副作用のない request を送る。
- Prod は GitHub Actions の deploy 後チェックに加えて、外部 synthetic monitoring で定期的に `/health` を監視する。
- `/health?ready=1` に GCS や Firebase など DB 以外の依存確認を追加する。

## 受け入れ条件

- Stage backend deploy 後に API / Worker の smoke test が走る計画になっている。
- Prod backend deploy 後に API / Worker の smoke test が走る計画になっている。
- Prod frontend deploy 後に Frontend の smoke test が走る計画になっている。
- Prod の smoke test に write / destructive 操作を含めない方針が明文化されている。
