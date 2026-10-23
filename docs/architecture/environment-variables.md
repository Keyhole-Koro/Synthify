# 環境変数・機密情報の管理方針

本プロジェクトでは、情報の機密性と管理の容易さを両立するため、情報の種類に応じて 3 つの場所に分けて管理します。

## 1. 管理の 3 層構造

| 分類 | 格納場所 | 内容の例 | 管理方法 |
| :--- | :--- | :--- | :--- |
| **デプロイ用機密情報** | **GitHub Actions Secrets** | WIF プロバイダー ID、サービスアカウント、GCP プロジェクト ID | GitHub リポジトリの設定画面で管理 |
| **アプリ用機密情報** | **Google Secret Manager** | DB 接続文字列、Webhook シークレット | Terraform で器を作成し、値は `gcloud` コマンド等で手動投入 |
| **アプリ用一般設定** | **Terraform (`.tfvars`)** | モデル名、CORS 許可ドメイン、各種 ID | `terraform/tfvars/` 配下のファイルでコード管理 |

---

## 2. 必要な環境変数一覧

### A. アプリケーション機密情報 (Secret Manager)
これらの値は Cloud Run 起動時に Secret Manager から自動的にマウントされます。

| 変数名 | 説明 | 備考 |
| :--- | :--- | :--- |
| `DATABASE_DSN` | CockroachDB/Postgres の接続文字列 | CockroachDB Serverless の DSN。例: `postgresql://<user>:<password>@<cluster>.cockroachlabs.cloud:26257/<db>?sslmode=verify-full` |
| `STRIPE_SECRET_KEY` | Stripe のシークレットキー | `sk_test_...` 等 |
| `STRIPE_WEBHOOK_SECRET` | Stripe Webhook の署名検証用 | |
| `INTERNAL_WORKER_TOKEN` | API と Worker 間の認証トークン | 任意の共有文字列 |
| `NEW_RELIC_LICENSE_KEY` | New Relic のライセンスキー | (利用する場合) |
| `GCS_SIGNING_PRIVATE_KEY` | GCS Signed URL 署名用サービスアカウント秘密鍵 | IAM Credentials `signBlob` を使う場合は不要 |

### B. 一般設定 (Terraform / tfvars)
機密ではないが、環境ごとに異なる可能性のある設定です。

| 変数名 | 説明 | デフォルト値の例 |
| :--- | :--- | :--- |
| `GEMINI_MODEL` | 使用する AI モデル | `gemini-1.5-flash` |
| `eval_image` | LLM eval Cloud Run Job の container image | CD が `-var` で SHA tag image を渡す |
| `eval_schedule` | LLM eval Cloud Run Job を起動する cron | `0 4 * * *` |
| `eval_time_zone` | eval Scheduler の timezone | `Asia/Tokyo` |
| `maintenance_schedule` | API の定期 sweep (job auto-resume / stuck job 検知) を起動する cron | `*/5 * * * *`。New Relic の `JobStuck` アラートが 5 分窓を前提にしているので、変えるなら `stuck_jobs` condition の `aggregation_window` も合わせる |
| `maintenance_time_zone` | maintenance Scheduler の timezone | `Asia/Tokyo` |
| `SYNTHIFY_MAINTENANCE_OIDC_AUDIENCE` | sweep 用 OIDC token の audience | Terraform が `services/api` の locals で生成し、Scheduler 側と同じ値を注入する。手で設定するものではない |
| `SYNTHIFY_MAINTENANCE_SCHEDULER_SERVICE_ACCOUNTS` | sweep を叩ける呼び出し元 SA (カンマ区切り) | Terraform が `synthify-maint-sched-<env>` を注入する。**空だと API は sweep endpoint を mount しない** (fail closed) ので、sweep が止まったらまずここを見る |
| `CORS_ALLOWED_ORIGINS` | フロントエンドの URL | `https://synthify.keyhole.work` |
| `STRIPE_PRO_PRICE_ID_JPY` | JPY 建て Stripe 価格 ID（カンマ区切りで複数可） | `price_...` |
| `STRIPE_PRO_PRICE_ID_USD` | USD 建て Stripe 価格 ID（カンマ区切りで複数可） | `price_...` |
| `BILLING_SUCCESS_URL` | 決済成功後のリダイレクト先 | `/billing/success` |
| `NEW_RELIC_APP_NAME` | New Relic 上のアプリケーション名 | `synthify-api` |
| `GCS_UPLOAD_ISSUER` | Upload URL 発行方式。`fake` または `signed` | ローカルは未設定/`fake`、本番 GCS は `signed` |
| `GCS_BUCKET` | アップロード先 GCS バケット | `synthify-uploads` |
| `GCS_SIGNING_SERVICE_ACCOUNT_EMAIL` | Signed URL の署名主体 | `signBlob` 利用時はこの SA に `iam.serviceAccounts.signBlob` 権限が必要 |
| `GCS_SIGNED_URL_TTL_MINUTES` | Signed URL の有効期限 | `15` |
| `SYNTHIFY_ADMIN_USER_EMAILS` | 管理者の email (カンマ区切り) | 追加権限。空で admin なし |
| `SYNTHIFY_ALLOWED_USER_EMAILS` | アクセス許可 email (カンマ区切り) | **非空なら API は許可 email のみ**。空で無制限。stage を 1 アカウントに絞る用途。CD は GitHub Environment 変数 `ALLOWED_USER_EMAILS` から `-var` で注入し、未設定なら tfvars のフォールバックを使う |
| `NEXT_PUBLIC_NEW_RELIC_BROWSER_LICENSE_KEY` | New Relic Browser の browser license key | frontend から参照される公開値 |
| `NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID` | New Relic Browser application ID | frontend から参照される公開値 |
| `NEXT_PUBLIC_NEW_RELIC_BROWSER_ACCOUNT_ID` | New Relic account ID | New Relic Browser snippet の `loader_config` から転記 |
| `NEXT_PUBLIC_NEW_RELIC_BROWSER_TRUST_KEY` | New Relic trust key | New Relic Browser snippet の `loader_config` から転記 |
| `NEXT_PUBLIC_NEW_RELIC_BROWSER_AGENT_ID` | New Relic Browser agent ID | 未設定なら application ID を使う |

---

## 3. 環境ごとの設定手順

### ローカル開発環境
ルートディレクトリの `.env` ファイルで一括管理します。
1. `.env.example` を `.env` にコピー
2. 必要なキー（Stripe 等）を記入
3. `docker compose up` で起動

### ステージング・本番環境 (GCP)
#### ① Secret Manager への値の投入
Terraform でリソースを作成した後、以下のコマンドで値を設定します。
```bash
# CockroachDB Serverless の DSN を設定する場合
echo -n "postgresql://<user>:<password>@<cluster>.cockroachlabs.cloud:26257/<db>?sslmode=verify-full" \
  | gcloud secrets versions add synthify-database-dsn --data-file=-

# New Relic のライセンスキーを設定する場合
echo -n "YOUR_NEW_RELIC_LICENSE_KEY" \
  | gcloud secrets versions add synthify-new-relic-license-key --data-file=-
```

#### ② Terraform への設定
`terraform/tfvars/prod.tfvars` (または `stage.tfvars`) に一般設定を記述し、`terraform apply` を実行します。
```hcl
cors_allowed_origins = "https://synthify.keyhole.work"
new_relic_app_name   = "synthify-api"
```

#### ③ GitHub Actions の設定
GitHub の `Settings > Secrets and variables > Actions` から、デプロイに必要な情報を登録します。
- `GCP_WIF_PROVIDER`
- `GCP_WIF_SA_EMAIL`
- `GCP_PROJECT_ID` (Variables)
- `GCP_REGION` (Variables)

---

## 4. 新しい変数を追加する場合のルール

1. **機密情報（パスワード、キー）の場合:**
   - `terraform/services/platform/main.tf` の `local.api_secrets` または `local.worker_secrets` に名前を追加。
   - `terraform/services/api/main.tf` などの `secret_env_vars` に定義を追加。
2. **一般設定の場合:**
   - `terraform/environments/variables.tf` に変数を定義 (stage/prod 共通の単一 root)。
   - `terraform/services/api/main.tf` などの `env_vars` に渡し、`tfvars/<env>.tfvars` に値を記述。
3. **ドキュメントの更新:**
   - 本ファイル (docs/architecture/environment-variables.md) を更新。
