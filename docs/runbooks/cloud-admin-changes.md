# admin change outside CD

| | |
| --- | --- |
| 条件 | GCP の Admin Activity の監査ログに、CD のデプロイ用 SA と Google の主体以外による次の操作が 1 件。IAM のポリシーの変更、SA の鍵の作成とアップロード、Cloud Run のサービスとジョブの作成・更新・削除、Secret Manager の secret の作成と版（値）の追加、Workload Identity のプロバイダの作成と更新。同じ主体・操作・対象の通知は 5 分に 1 回まで |
| 通知先 | メール（GitHub の environment secret `PLATFORM_ALERT_EMAIL`） |
| 定義 | `terraform/services/platform_alerts/main.tf`（`admin_changes`） |

## 意味

権限、鍵、動いているサービス、秘密の値のどれかが、CD（`deploy-backend.yml` の `terraform apply`）を通らずに変わった。通知には、誰が（principal）、何を（method）、どれに（resource_name）が入る。

- 除外するのは、デプロイ用 SA（`deployer_principal`。`github-deploy@<プロジェクト>.iam.gserviceaccount.com`）、Google が管理するサービスエージェント（`service-<番号>@gcp-sa-*.iam.gserviceaccount.com`、`<番号>@cloudservices.gserviceaccount.com`）、Google の内部の主体（`*@system.gserviceaccount.com`）。
- デプロイ用 SA 自身の操作は鳴らない。デプロイ用 SA を不正に使われても、この通知では気づけない。デプロイ用 SA になれる条件（Workload Identity のプロバイダ）を CD の外で変えれば鳴る。
- 主体が記録されていない操作も鳴る。
- 人が手で行う決まった作業でも鳴る。secret に値を入れる（`gcloud secrets versions add`）、Cloud Run を前の版に戻す、`terraform/manual/` を apply するなど。

## 見るもの

```bash
PROJECT=synthify-stage-491705   # prod は synthify-491705
# 直近 2 時間の、デプロイ用 SA 以外による管理の操作
gcloud logging read "logName=\"projects/$PROJECT/logs/cloudaudit.googleapis.com%2Factivity\" AND NOT protoPayload.authenticationInfo.principalEmail=\"github-deploy@$PROJECT.iam.gserviceaccount.com\"" \
  --project "$PROJECT" --freshness 2h --limit 50 \
  --format 'table(timestamp, protoPayload.authenticationInfo.principalEmail, protoPayload.methodName, protoPayload.resourceName)'
```

- IAM の変更なら、そのエントリを `--format json` で開き、`protoPayload.serviceData.policyDelta.bindingDeltas` で、足した・外したメンバーとロールを見る。
- 主体が自分なら、何のための操作だったかを思い出す。

## 対処

- 自分の作業なら、何をしたかを記録して閉じる。
  - Cloud Run の変更は、次の CD の apply で Terraform の状態に戻る。
  - IAM は、Terraform が付けた権限を外した場合は次の apply で付け直される。Terraform の外で足したメンバー、SA の鍵、Terraform の外で作った secret と版は残る。要らなければ消す。
- 心当たりが無ければ、侵害として扱う。
  1. 主体を止める。人のアカウントならパスワードを変えてセッションを切る。SA なら、鍵を無効にするか SA を無効にする（実行用の SA を無効にするとサービスが止まる）。
  2. 作られた SA の鍵を無効にして消す（`gcloud iam service-accounts keys disable` / `delete`）。
  3. 足された IAM のメンバーを外す。
  4. secret に足された版を無効にする（`gcloud secrets versions disable`）。秘密の値の読み出しは Data Access の監査ログに出るが、有効にしていない。読まれたものとして、その secret の値を作り直す（DB のパスワード、Stripe の鍵、`internal-worker-token` など）。
  5. Cloud Run を CD で出し直し、Terraform の状態に戻す。
  6. 同じ主体のほかの操作を、Admin Activity の監査ログで洗い出す。
- 古い形式のサービスエージェント（例: `service-<番号>@serverless-robot-prod.iam.gserviceaccount.com`）で鳴ったら、Google の自動の操作なので、`main.tf` の除外に足す。

## 閉じる条件

誰の、何のための操作かが分かり、Terraform とのずれを CD で揃えた。侵害なら、上の対処を終えた。インシデントは最後の一致から 30 分で自動で閉じるが、確かめ終わるまで続ける。
