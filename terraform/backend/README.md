# Backend

Terraform state backend (GCS) の partial config を置くディレクトリ。

単一 root (`terraform/environments`) を stage/prod で使い回すため、backend は
`-backend-config` で切り替える:

```sh
cd terraform/environments

# stage
terraform init -backend-config=../backend/stage.hcl
terraform apply -var-file=../tfvars/stage.tfvars

# prod
terraform init -reconfigure -backend-config=../backend/prod.hcl
terraform apply -var-file=../tfvars/prod.tfvars
```

`*.hcl` は gitignore 済み。state バケットと backend config は
`scripts/bootstrap-tfstate.sh` で作成する (chicken-and-egg を避けるため Terraform 管理外)。
