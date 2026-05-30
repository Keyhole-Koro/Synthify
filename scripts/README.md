# Scripts

このディレクトリには、ローカル開発、インフラ構築、CI/CD などで使用される各種スクリプトが含まれています。

## データベース・マイグレーション
* **`apply-db-migrations.sh`**: `golang-migrate` を使用して `db/migrations` 以下の SQL をターゲットの DB に適用します。
  * **利用箇所**: 主に GitHub Actions (`deploy-backend.yml`) のデプロイパイプライン内で自動実行されます。
* **`reset_db.sh`**: ローカルの PostgreSQL Docker コンテナとボリュームを削除し、DB の状態をリセットします。
  * **利用箇所**: ローカル開発時に、開発者が手動でデータベースをクリーンアップしたい場合に使用します。
* **`run_explain.sh`**: `db/queries` 内の各 SQL に対して `EXPLAIN` を実行し、結果をテキストファイルに出力します。
  * **利用箇所**: ローカル開発時に、開発者がクエリのパフォーマンスや実行計画を手動で確認したい場合に使用します。

## ローカル開発・テスト
* **`get-token.sh`**: ローカルの Firebase Auth エミュレータからテスト用の ID トークンを取得します。
  * **利用箇所**: ローカル開発時に、開発者が手動でAPIを叩く際（`curl` など）の Authorization ヘッダー生成に使用します。
* **`seed_gcs.sh`**: `demo_docs/` 内のデモファイルをローカルの GCS エミュレータ (`fake-gcs-server`) にアップロードします。
  * **利用箇所**: ローカルでの動作確認時や、`worker_demo.sh` を実行する前の事前準備として開発者が手動で実行します。
* **`worker_demo.sh`**: シードドキュメントを用いてローカルの Worker にジョブを送信し、動作確認を行います。
  * **利用箇所**: ローカル環境で Worker の動作（LLMパイプライン等）を開発者が手動でテスト・デモする際に使用します。
* **`smoke-test.sh`**: デプロイ後の各エンドポイント (API, Worker, Frontend) に対して、簡易的な稼働確認 (スモークテスト) を実行します。
  * **利用箇所**: GitHub Actions (`deploy-backend.yml`, `deploy-frontend.yml`) のデプロイ直後の検証ステップとして自動実行されます。

## インフラ・コード生成
* **`manage-env.sh`**: 指定した環境 (stage, prod) に対して Terraform のコマンド (plan, apply 等) を実行するためのラッパーです。
  * **利用箇所**: プロジェクトルートの `Makefile` (`make stage-up`, `make prod-plan` など) から呼び出されます。
* **`bootstrap-tfstate.sh`**: Terraform の tfstate を保存するための GCS バケットを初期構築します。
  * **利用箇所**: GitHub Actions (`deploy-backend.yml`) の Terraform 実行前、および `Makefile` (`make bootstrap-stage` など) から呼び出されます。
* **`generate-firestore-types.mjs`**: Firestore の JSON スキーマ (`job-status.schema.json`) から、TypeScript (Web 用) と Go (Platform 用) の型定義を自動生成します。
  * **利用箇所**: `apps/web/package.json` の npm scripts (`npm run generate:firestore-types`) として登録されており、スキーマ変更時に実行されます。
