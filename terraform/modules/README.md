# Modules

再利用可能な Terraform module を置くディレクトリ。

例:

- `cloud_run_service`
- `cloud_tasks_queue`
- `gcs_bucket`

サービスアカウントは module にせず、`terraform/manual/iam` が直接作る（デプロイ用 SA には作れない）。
