# Services

サービス単位の Terraform 構成を置くディレクトリ。

例:

- `api`
- `worker`
- `tasks`
- `secrets`

`api` と `worker` は Cloud Run の runtime env を受け取る。
秘密値は Secret Manager 名を Terraform 変数で受け、`secret_env_vars` 経由で注入する。
