# アラート

何が起きたら、どこに知らせるかの一覧。鳴ったときの手順は [runbooks](runbooks/) にある。

## 通知の経路

| 経路 | 何を送るか | 設定 |
| --- | --- | --- |
| Discord（環境ごとの webhook） | New Relic のアラート。アプリのエージェントが送ったものだけを見る | `terraform/services/monitoring/`。GitHub の environment secret `DISCORD_ALERT_WEBHOOK_URL` |
| メール | Cloud Monitoring の外形監視と、CD 以外による管理の操作。アプリ、New Relic、Discord の webhook のどれが止まっても届く | `terraform/services/platform_alerts/`。GitHub の environment secret `PLATFORM_ALERT_EMAIL` |
| メール（billing account の管理者とユーザー） | 予算 | `terraform/manual/billing/`（手で apply する） |
| Discord（環境ごとの webhook） | GitHub Actions の readiness の確認（補助） | `.github/workflows/readiness-monitor.yml` |
| Discord（リポジトリの webhook） | CI とデプロイの失敗 | `.github/workflows/notify-discord.yml`。リポジトリの secret `DISCORD_WEBHOOK_URL` |

- `PLATFORM_ALERT_EMAIL` が空でも、Cloud Monitoring のアラートは作られる。通知先が無いので、コンソールのインシデントにしか出ない。デプロイのログに warning が出る。
- アドレスは tfvars に書かない。リポジトリも Actions のログも公開なので、GitHub の variable ではなく secret に置く。
- readiness の cron は 15 分ごとの設定だが、GitHub が遅らせるので実際は 3.7〜9.7 時間おき（2026-11-10〜21）。60 日コミットが無いと GitHub が止める。外形監視の代わりにはならない。Discord に「API が落ちた」を送れるのはこれだけなので、補助として残している。

## Cloud Monitoring（メール）

`terraform/services/platform_alerts/main.tf`。環境ごとに、その環境のプロジェクトに作る。

| 名前 | 鳴る条件 | 手順書 |
| --- | --- | --- |
| web unreachable | `web_base_url` の `/` の外形監視（5 分ごと、アジア・ヨーロッパ・米国の 3 地点）が、2 地点以上で 10 分続けて失敗。200 でも `<title>Synthify</title>` が無ければ失敗 | [uptime](runbooks/uptime.md) |
| api unreachable | API の `/health?ready=1` の外形監視（同じ間隔と地点）。`X-Synthify-Readiness-Key` に `READINESS_MONITOR_KEY` を付ける。API が DB に届かなければ 503、鍵が違えば 401 で失敗 | [uptime](runbooks/uptime.md) |
| admin change outside CD | CD のデプロイ用 SA と Google の主体以外が、IAM のポリシー、SA の鍵、Cloud Run のサービスとジョブ、Secret Manager の secret と版、Workload Identity のプロバイダを変えた（Admin Activity の監査ログ。1 件で） | [cloud-admin-changes](runbooks/cloud-admin-changes.md) |

開いたときと閉じたとき（回復から 30 分）にメールが届く。通知の本文に手順書のリンクが付く。

## 予算（メール）

`terraform/manual/billing/`。CD では apply しない（[budget](runbooks/budget.md)）。

| 名前 | 鳴る条件 | 手順書 |
| --- | --- | --- |
| `synthify-stage-budget`、`synthify-prod-budget` | その月の費用が環境ごとの予算（1,000 円）の 50%・90%・100% を超えた。月末の予測が 100% を超えた | [budget](runbooks/budget.md) |

## New Relic（Discord）

`terraform/services/monitoring/`。しきい値は `variables.tf` の既定値。critical と warning の 2 つのポリシーに分かれ、どちらも Discord に届く。

| 名前 | 鳴る条件 | 重さ |
| --- | --- | --- |
| API 5xx error rate | API の 5xx が 5 分で 5% を超えた | critical |
| API p95 response time | API の p95 が 5 分で 3 秒を超えた | warning |
| API auth error spike | API の 401・403 が 5 分で 100 件を超えた | warning |
| Billing/Stripe error spike | `billing.` で始まるエラーが 5 分で 3 件を超えた | critical |
| Browser JS error spike | 画面の JS のエラーが 5 分で 10 件を超えた | warning |
| Job failure spike | `JobFailed` が 5 分で 5 件を超えた | critical |
| Stuck job spike | `JobStuck` のジョブが 1 件でも出た | critical |
| Job notification failure spike | `NotificationFailed` が 5 分で 3 件を超えた | warning |
| LLM rate-limit/quota pressure | `LLMThrottled`（`rate_limited`）が 5 分で 30 件を超えた | warning |
| Upload rejection spike | `UploadRejected` が 5 分で 50 件を超えた | warning |
| Worker 5xx error rate | worker の 5xx が 5 分で 5% を超えた。worker から 15 分何も届かない（信号断） | critical |
| Worker p95 response time | worker の p95 が 5 分で 10 秒を超えた | warning |

## メールが届くかを確かめる

デプロイの後、環境ごとに一度鳴らして、メールが届くのを見る。迷惑メールに入っていないかも見る。

CD 以外の操作で鳴るアラートを使うのが簡単。secret を 1 つ作って消すと、`admin change outside CD` が鳴る（作成だけが条件で、消すのは条件に入っていない）。

```bash
PROJECT=synthify-stage-491705   # prod は synthify-491705
gcloud secrets create synthify-alert-drill --project "$PROJECT" --replication-policy=automatic
# 数分でメールが届く。届いたら消す
gcloud secrets delete synthify-alert-drill --project "$PROJECT" --quiet
```

外形監視のアラートは、わざと落とさないと鳴らない。届く経路は同じなので、上で確かめれば足りる。外形監視が動いているかは、Cloud Monitoring の「Uptime checks」で、2 つのチェックが緑になっているかを見る。
