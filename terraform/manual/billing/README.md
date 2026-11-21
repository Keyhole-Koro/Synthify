# 予算（手で apply する）

stage と prod のプロジェクトごとに、月の予算（`google_billing_budget`）を 1 つずつ作る。鳴ったときの手順は [docs/runbooks/budget.md](../../../docs/runbooks/budget.md)。

| 予算 | 対象のプロジェクト | 月の額 | 知らせる時 |
| --- | --- | --- | --- |
| `synthify-stage-budget` | `synthify-stage-491705` | 1,000 円 | 実績が 50%・90%・100%、月末の予測が 100% |
| `synthify-prod-budget` | `synthify-491705` | 1,000 円 | 同上 |

通知は billing account の管理者とユーザーにメールで届く（予算の既定）。

## CD で apply しない理由

予算は billing account のリソースで、プロジェクトのロールでは作れない。billing account の Billing Account Administrator か Costs Manager が要る。

デプロイ用 SA（`github-deploy@<プロジェクト>`）は予算を作れない（2026-11-21 時点）。

- プロジェクトのロールは 13 個で、billing のロールは無い。
- billing account のロールが付いているかは読めなかった（billing account の IAM を読む Cloud Billing API が、オーナーのアカウントのどのプロジェクトでも有効でなく、読むには API を有効にする変更が要る）。
- Budget API（`billingbudgets.googleapis.com`）が stage・prod のどちらのプロジェクトでも無効だった。SA の呼び出しは SA のプロジェクトで数えるので、この SA から予算を作れたことは無い。

billing account のロールを付ければ CD でも作れるが、その SA が同じ billing account のほかのプロジェクトの予算も読み書きできるようになる。公開リポジトリから動くデプロイの権限を広げないために、ここは人が一度だけ apply する。CD（`deploy-backend.yml`）は `terraform/environments` しか apply しない。

## apply の仕方

前提:

- 自分のアカウントに、billing account の Billing Account Administrator か Costs Manager。
- 自分のアカウントに、quota project（`quota_project_id`、既定は `synthify-491705`）の `serviceusage.services.use`（オーナーなら持っている）。
- quota project で `billingbudgets.googleapis.com` が有効。`services/platform` の `required_services` にあるので、この変更を含むデプロイの後なら有効になっている。その前に apply するなら、先に `gcloud services enable billingbudgets.googleapis.com --project synthify-491705`。

```bash
gcloud auth application-default login

cd terraform/manual/billing
# billing account の ID はコミットしない
BILLING_ACCOUNT="$(gcloud billing projects describe synthify-491705 --format='value(billingAccountName)')"
BILLING_ACCOUNT="${BILLING_ACCOUNT#billingAccounts/}"

# state は prod の state バケットの、CD とは別の prefix に置く
terraform init -backend-config="bucket=synthify-491705-tfstate-prod" -backend-config="prefix=synthify/billing"
terraform plan  -var="billing_account_id=${BILLING_ACCOUNT}"
terraform apply -var="billing_account_id=${BILLING_ACCOUNT}"
```

確かめる:

```bash
gcloud billing budgets list --billing-account="${BILLING_ACCOUNT}" --billing-project=synthify-491705 \
  --format='table(displayName, amount.specifiedAmount.units, budgetFilter.projects.list())'
```

額やしきい値を変えるときは `variables.tf` の既定値を変えて、同じ手順で apply する。
