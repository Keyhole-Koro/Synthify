# Stage環境管理スクリプト仕様

## 背景
現在のStage環境はTerraformで管理されていますが、以下の課題から環境の構築・破棄の手順が複雑になっています。
1. **2パスアプライの必要性**: WorkerとAPIの間に循環参照（WorkerはAPIのURLを知る必要があり、APIのCloud RunはWorkerより後に作られるか、もしくは同時に作られる過程でURLが確定する）があります。最初は `api_base_url` を空にしてApplyし、APIが作成された後にそのURLをWorkerに渡して再度Applyする必要があります。
2. **Secret Managerの要件**: Cloud Runが正常に起動するためには、参照するSecret Managerのシークレットに最低1つのバージョン（値）が存在している必要があります。
3. **頻繁なDBスキーマ変更**: 開発段階においてデータベース（外部マネージドのCockroachDBを使用）のスキーマが頻繁に変わるため、マイグレーションファイルによる管理ではなく、素早くスキーマを壊してローカルの `db/init/` 状態から作り直す仕組みが求められています。

## 提供するコマンド
これらの課題を解決するため、`Makefile` に以下のターゲットを追加し、開発者が単一のコマンドで操作できるようにします。

- `make infra-stage-plan`: TerraformのPlanを実行します。
- `make infra-stage-up`: Stage環境を構築します（2パスアプライとシークレットの初期化を自動化）。
- `make infra-stage-down`: Stage環境を完全に破棄します（GCSバケットのクリーンアップを含む）。
- `make db-stage-reset`: Stage環境のデータベース（CockroachDB）のスキーマを全て削除し、最新の初期化スクリプトで再構築します。

## スクリプトの仕様 (`scripts/manage-stage.sh`)
上記コマンドの実体となるBashスクリプトの詳細な仕様です。

### 1. `up` コマンド
1. `terraform init` を実行します。
2. 1回目の `terraform apply` を `-auto-approve` で実行します（Pass 1）。
3. Secret Managerの必須シークレット（`database-url`, `gemini-api-key` など）のバージョンを確認し、存在しない（有効なバージョンがない）場合はプレースホルダー値（例: `placeholder-change-me`）を投入します。
4. TerraformのOutputから `api_uri` を取得します。
5. `-var="api_base_url=$API_URI"` を付与して2回目の `terraform apply` を実行し、Workerに変更を適用します（Pass 2）。

### 2. `down` コマンド
1. 実行前に確認プロンプト（`y/N`）を表示します。
2. TerraformのOutputから `uploads_bucket_name` を取得します。
3. `gsutil -m rm -rf "gs://$BUCKET_NAME/*"` を実行し、GCSバケットを空にします（Terraformは中身のあるGCSバケットをDestroyできないため）。
4. `terraform destroy -auto-approve` を実行します。

### 3. `plan` コマンド
1. `terraform init` と `terraform plan` を実行します。

### 4. `reset-db` コマンド
1. 実行前に確認プロンプト（`y/N`）を表示します。
2. TerraformのOutputから `project_id` を取得します。
3. `gcloud secrets versions access latest --secret="synthify-database-url-stage" --project="<PROJECT_ID>"` を実行し、CockroachDBデータベースの接続文字列を取得します。
4. 取得した接続文字列を使用して `psql` でデータベースに接続し、以下のコマンドで既存スキーマとデータを全て削除・再作成します。
   ```sql
   DROP SCHEMA public CASCADE;
   CREATE SCHEMA public;
   ```
5. `db/init/*.sql` のスクリプト群をアルファベット順に `psql` で流し込み、最新のスキーマとシードデータを適用してデータベースを再構築します。

## Terraformの変更点
管理スクリプトが動的にGCPプロジェクトの情報を取得できるよう、`terraform/stage/outputs.tf` に以下の出力を追加します。
- `output "project_id"`
- `output "environment"`
