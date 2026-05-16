# Terraform Layout

Terraform の構成用ディレクトリ。

```
terraform/
  backend/    # state backend 設定 (GCS bucket)
  modules/    # 再利用 module
  services/   # service 単位の構成 (platform/api/worker)
  stage/      # stage 環境
  prod/       # prod 環境
  tfvars/     # 環境別 tfvars サンプル
```

## 構成方針

- **DB は CockroachDB Serverless (外部マネージド)**。Cloud SQL は作らない。CockroachDB の接続文字列を
  Secret Manager の `synthify-database-url-<env>` に手動で投入する。
- **Cloud Run × 2**: `synthify-api-<env>` と `synthify-worker-<env>`。
  worker は INTERNAL ingress、api は public。
- **環境変数は Terraform が全管理**。CI/CD は image だけ差し替える(`gcloud run deploy --image`)。
  env を変えたい時は terraform apply。
- **Secret は platform module が一括作成**。version (値) は手動で投入。

## 初回セットアップ

1. GCS state bucket を作って `backend/<env>.hcl` を用意
2. `tfvars/<env>.tfvars` を埋める (api_base_url は空のまま)
3. `terraform -chdir=terraform/<env> init -backend-config=../backend/<env>.hcl`
4. `terraform -chdir=terraform/<env> apply -var-file=../tfvars/<env>.tfvars`
5. 作成された Secret Manager に値を投入 (下記)
6. CI でイメージを build & push、Cloud Run が起動
7. `terraform output -raw api_uri` を tfvars の `api_base_url` に書いて再 apply
   (worker → api コールバックを有効化)

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
# init
terraform -chdir=terraform/stage init \
  -backend-config=../backend/stage.hcl

# plan / apply
terraform -chdir=terraform/stage plan -var-file=../tfvars/stage.tfvars
terraform -chdir=terraform/stage apply -var-file=../tfvars/stage.tfvars

# outputs
terraform -chdir=terraform/stage output
```
