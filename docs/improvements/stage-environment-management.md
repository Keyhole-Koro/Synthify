# Stage環境管理スクリプト仕様

## 背景
現在のStage環境はTerraformで管理されていますが、以下の課題から環境の構築・破棄の手順が複雑になっています。
1. **2パスアプライの必要性**: WorkerとAPIの間に循環参照があります。最初は `api_base_url` を空にしてApplyし、APIが作成された後にそのURLをWorkerに渡して再度Applyする必要があります。
2. **Secret Managerの要件**: Cloud Runが正常に起動するためには、参照するSecret Managerのシークレットに最低1つのバージョン（値）が存在している必要があります。
3. **頻繁なDBスキーマ変更**: 開発段階においてデータベース（外部マネージドのCockroachDBを使用）のスキーマが頻繁に変わるため、マイグレーションファイルによる管理ではなく、素早くスキーマを壊してローカルの `db/init/` 状態から作り直す仕組みが求められています。

## 提供するコマンド
`Makefile` に以下のターゲットを追加します。

- `make infra-stage-plan`: Stage環境のTerraform Planを実行。
- `make infra-stage-up`: Stage環境の構築（2パスアプライとシークレット初期化）。
- `make infra-stage-down`: Stage環境の破棄（GCSクリーンアップ含む）。
- `make db-stage-reset`: Stage環境のCockroachDBをリセット。

## スクリプトの仕様 (`scripts/manage-stage.sh`)

### 1. `up` コマンド
1. `terraform init` を実行。
2. 1回目の `terraform apply` を実行。
3. 必須シークレットのバージョンを確認し、なければプレースホルダーを追加。
   - 対象: `synthify-database-dsn`, `synthify-gemini-api-key`, `synthify-internal-worker-token`, `synthify-stripe-secret-key`, `synthify-stripe-webhook-secret`, `synthify-new-relic-license-key`
4. Outputから `api_uri` を取得し、2回目の `terraform apply -var="api_base_url=$API_URI"` を実行。

### 2. `down` コマンド
1. 確認プロンプトを表示。
2. Outputから `uploads_bucket_name` を取得し、`gsutil -m rm -rf` でバケットを空にする。
3. `terraform destroy` を実行。

### 4. `reset-db` コマンド
1. 確認プロンプトを表示。
2. `gcloud secrets versions access latest --secret="synthify-database-dsn"` で接続文字列を取得。
3. `psql` で `DROP SCHEMA public CASCADE; CREATE SCHEMA public;` を実行。
4. `db/init/*.sql` を順次適用。

