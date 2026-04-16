# Terraform Layout

Terraform の構成用ディレクトリ。

```
terraform/
  backend/
  modules/
  services/
  stage/
  prod/
  tfvars/
```

- `backend/`: state backend 設定用
- `modules/`: 再利用 module
- `services/`: service 単位の構成
- `stage/`: stage 環境
- `prod/`: prod 環境
- `tfvars/`: 環境別 tfvars
