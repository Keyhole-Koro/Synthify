# 予算

| | |
| --- | --- |
| 条件 | その月の費用が、環境ごとの月の予算（`monthly_amount`、1,000 円）の 50%・90%・100% を超えた。月末の予測が 100% を超えた |
| 対象 | `synthify-stage-budget` は `synthify-stage-491705`、`synthify-prod-budget` は `synthify-491705` の費用だけ。クレジットを引いた後の額 |
| 通知先 | メール（billing account の管理者とユーザー。予算の既定） |
| 定義 | `terraform/manual/billing/`。CD では apply しない（[README](../../terraform/manual/billing/README.md)） |

## 前提

- 予算は billing account のリソースなので、作るには billing account のロール（Billing Account Administrator か Costs Manager）が要る。デプロイ用 SA には付けていないので、人が手で apply する。
- 同じ billing account に、プロジェクトを絞らない「¥300 Monthly Budget Alert」が手で作られている（Terraform の管理の外）。billing account 全体の費用で鳴る。
- Gemini API のキー（AI Studio）の費用は、キーを発行したプロジェクトの請求に載る。前払いのときは、残高を足した時点で費用になる。どちらもこの予算の対象外（未確認）。

## 見るもの

- 「お支払い」→「レポート」で、プロジェクトを絞り、SKU 別に見て、どのサービスで増えたか。
- Cloud Run が増えていれば、インスタンス数とリクエスト数。CPU が常に割り当てられる設定（`cpu_idle = false`）や `min_instance_count` を手で変えていないか（`terraform/modules/cloud_run_service`）。
- Vertex AI が増えていれば、worker のジョブの数と、ジョブごとの LLM の呼び出し（New Relic の `LLMThrottled`、`JobFailed`）。
- Cloud Storage が増えていれば、uploads のバケットの大きさ。
- Artifact Registry が増えていれば、イメージの数。

## 対処

- 不正な利用なら、利用者を止める（stage なら `allowed_user_emails`）。必要なら worker を止める（Cloud Tasks のキューを一時停止する: `gcloud tasks queues pause synthify-pipeline-<env> --location asia-northeast1 --project <project>`）。
- 手作業による設定の変更なら、CD で Terraform を再適用して戻す。
- 予算が実態に合わないなら、`terraform/manual/billing/variables.tf` の `monthly_amount` を変えて apply する。

## 閉じる条件

増えた理由が分かり、月末の予測が予算内に戻った。
