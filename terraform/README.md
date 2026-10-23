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
  Secret Manager の `synthify-database-dsn` に手動で投入する。
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

`synthify-<key>` という命名。stage と prod は別 GCP project の Secret Manager に同じ名前で作成する。

| Key | 内容 | 用途 |
|---|---|---|
| `database-dsn` | CockroachDB Serverless の接続文字列 (`postgresql://...`)。admin 権限のユーザ (migration もこれで流す) | api / worker |
| `monitor-database-dsn` | 上と**同じ DB** への接続文字列。ただしユーザは read-only の `monitor` ロール (下記) | monitor |
| `internal-worker-token` | worker → api 内部通信トークン (適当なランダム文字列) | api / worker |
| `gemini-api-key` | Gemini API キー | worker / eval |
| `stripe-secret-key` | Stripe Secret Key (`sk_live_...`) | api |
| `stripe-webhook-secret` | Stripe Webhook Signing Secret | api |
| `new-relic-license-key` | New Relic Ingest License Key | api / worker |

Cloud Run が `latest` を参照するので、**この表の全部が最低 1 version 必要**。
1 つでも version が無いと CD の `Verify all secrets have a version` が
image の build にも migration にも進まずに落ちる (`deploy-backend.yml`)。
実際に使わない連携があるなら placeholder を入れておく:

```bash
printf 'unused' | gcloud secrets versions add synthify-stripe-secret-key --data-file=-
```

ただし placeholder を入れてよいのは**本当に使っていない**ものだけ。値が空のまま
Cloud Run が「起動には成功するが動かない」状態になるのが一番厄介な壊れ方で、
過去に stage はこれで落ちている。

### `monitor-database-dsn` の作り方

monitor ダッシュボードは API を経由せず Postgres を直接読む。参照先は
**api / worker と同じクラスタ・同じ database** で、違うのは接続ユーザだけ。
DB レイヤで read-only に縛るため、専用の `monitor` ロールで接続する
(`terraform/services/monitor/main.tf` の `MONITOR_DATABASE_URL`)。

`monitor` ロール自体は migration `0014_monitor_role` が
`CREATE ROLE IF NOT EXISTS monitor LOGIN` で作り、参照してよいテーブルと view にだけ
`GRANT SELECT` する (eval 系は `0024_eval_views_grants`)。ただし
**パスワードは migration では設定していない**ので、環境ごとに一度だけ手で入れる:

```bash
# 1. monitor ロールにパスワードを設定 (admin DSN = synthify-database-dsn で接続)
#    パスワードは任意の強いランダム文字列
psql "$ADMIN_DSN" -c "ALTER ROLE monitor WITH PASSWORD '<generated>'"

# 2. その資格情報で DSN を組んで Secret に投入。
#    ホスト / database 名 / sslmode は database-dsn と同じものを流用し、
#    user と password だけ差し替える。
printf 'postgresql://monitor:<generated>@<host>:<port>/<database>?sslmode=verify-full' \
  | gcloud secrets versions add synthify-monitor-database-dsn \
      --project <gcp-project> --data-file=-
```

stage と prod で別々に必要 (project は `synthify-stage-491705` /
`synthify-491705`)。ローカルの compose では `monitor@127.0.0.1` に
パスワード無しで繋ぐ既定値が入っているので、この手順は不要。

投入後は push し直さなくても、Actions の `Deploy Backend` を
`workflow_dispatch` で環境を選んで再実行すれば続きから流れる。

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
