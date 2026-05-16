# Terraform Layout

Terraform の構成用ディレクトリ。

```
terraform/
  backend/        # state backend 設定 (GCS, env 別 partial config)
  modules/        # 再利用 module
  services/       # service 単位の構成 (platform/api/worker)
  environments/   # 単一 root。stage/prod は tfvars + backend で切替
  tfvars/         # 環境別 tfvars
```

stage/prod でディレクトリは分けない。単一 root (`environments/`) を
`-backend-config` と `-var-file` で切り替えて再利用する。

## 構成方針

- **DB は CockroachDB Serverless (外部マネージド)**。Cloud SQL は作らない。CockroachDB の接続文字列を
  Secret Manager の `synthify-database-url-<env>` に手動で投入する。
- **Cloud Run × 2**: `synthify-api-<env>` と `synthify-worker-<env>`。
  worker は INTERNAL ingress、api は public。
- **環境変数は Terraform が全管理**。env 名は環境中立 (prefix/suffix なし)、
  値の出し分けは tfvars / Secret Manager / `locals.tf` の導出で行う。
- **tfvars は最小限**。導出可能なものは `environments/locals.tf` で組み立てる:
  - `uploads_bucket_name` = `<project_id>-synthify-uploads-<environment>`
  - `firebase_project_id` = `project_id`
  - `cors_allowed_origins` / `billing_*_url` = `web_base_url` から組立
  - `new_relic_app_name` = prod は `synthify-api`、他は `synthify-api-<env>`
  - `env` = prod は `production`、他は `environment` 値
  - 各々 tfvars で明示すれば override 可能 (空 => 導出)。
  tfvars の必須は実質 `project_id` / `region` / `environment` / `web_base_url` のみ。
- **image は CD が `-var` で注入**。tfvars に書かない。CD は build & push の後に
  実 SHA タグ URL を `terraform apply -var="api_image=..."` で渡す。
- **state バケットは CD が upsert**。`deploy-backend.yml` が init 前に
  `scripts/bootstrap-tfstate.sh` を実行 (無ければ作成、あれば設定再適用)。
- **Secret は platform module が一括作成**。version (値) は手動で投入。
- **署名付きアップロード URL**: `gcs_upload_issuer = "signed"` で実 GCS 署名 URL。
  private key は不要 (IAM SignBlob 経由)。API service account が自分自身に
  対し `roles/iam.serviceAccountTokenCreator` を持つよう Terraform が付与する。
- **内部サービストークン**: worker は `INTERNAL_WORKER_TOKEN` を読んで送信、
  API の auth middleware は `SYNTHIFY_INTERNAL_SERVICE_TOKEN` を期待する。
  両者を同じ `internal-worker-token` secret にバインドして整合させている。

## 初回セットアップ

1. state bucket を作成 (backend/<env>.hcl も自動生成される):

   ```bash
   make tfstate-stage PROJECT_ID=your-stage-project
   # または: bash scripts/bootstrap-tfstate.sh stage your-stage-project
   ```

2. `tfvars/<env>.tfvars` を用意。埋めるのは実質 4 項目だけ
   (`project_id` / `region` / `environment` / `web_base_url`)。
   残りは locals.tf が導出する。機密値は tfvars に書かず、Secret Manager に投入する。

3. init / apply (Pass 1: platform のみ。image はまだ無い):

   ```bash
   cd terraform/environments
   terraform init -reconfigure -backend-config=../backend/stage.hcl
   terraform apply -var-file=../tfvars/stage.tfvars -target=module.platform
   ```

4. 作成された Secret Manager に値を投入 (下記)
5. image を build & push (CD が自動。手動なら docker push)
6. 全体 apply。image を `-var` で渡す:

   ```bash
   terraform apply -var-file=../tfvars/stage.tfvars \
     -var="api_image=<registry>/api:<tag>" \
     -var="worker_image=<registry>/worker:<tag>"
   ```

7. `terraform output -raw api_uri` を `-var="api_base_url=..."` で再 apply
   (worker → api コールバックを有効化)

`make infra-stage-up` / `make infra-prod-up` が 1〜6 のうち
init→apply→placeholder secret→2回目 apply を自動化する。

## Secret Manager に投入する値

`synthify-<key>-<env>` という命名。`<env>` は `stage` または `prod`。

| Key | 内容 | 必須 |
|---|---|---|
| `database-url` | CockroachDB Serverless の接続文字列 (`postgresql://...`) | ✅ |
| `gemini-api-key` | Gemini API key | ✅ |
| `internal-worker-token` | worker → api 内部通信トークン (適当なランダム文字列) | ✅ |
| `stripe-secret-key` | Stripe Secret Key (`sk_live_...`) | Stripe 使う時 |
| `stripe-webhook-secret` | Stripe Webhook Signing Secret | Stripe 使う時 |
| `new-relic-license-key` | New Relic Ingest License Key | NR 使う時 |

未使用の secret も Cloud Run が `latest` を参照するので**最低 1 version は必要**。
未使用なら placeholder 文字列を入れておく:

```bash
printf 'unused' | gcloud secrets versions add synthify-stripe-secret-key-stage --data-file=-
```

## 主要コマンド

```bash
cd terraform/environments

# stage
terraform init -reconfigure -backend-config=../backend/stage.hcl
terraform plan  -var-file=../tfvars/stage.tfvars
terraform apply -var-file=../tfvars/stage.tfvars
terraform output

# prod (backend を切り替えてから)
terraform init -reconfigure -backend-config=../backend/prod.hcl
terraform apply -var-file=../tfvars/prod.tfvars
```

> ⚠️ env を切り替えるたび `terraform init -reconfigure -backend-config=...`
> を必ず実行する (state を取り違えないため)。
