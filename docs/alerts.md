# アラート

何が起きたら、どこに知らせるかの一覧。鳴ったときの手順は[手順書](runbooks/README.md)にある。定義は Terraform（`terraform/services/monitoring/`、`terraform/services/platform_alerts/`、`terraform/manual/billing/`）と `.github/workflows/` を正とする。

通知の経路は 5 つある。Discord が 3 つ、メールが 2 つ。

- New Relic のアラート（Discord）: 環境ごとのポリシー `synthify-critical-<env>` と `synthify-warning-<env>`。New Relic の workflow が、issue の題と URL を Discord の webhook に送る。webhook の URL は GitHub の Environment の secret `DISCORD_ALERT_WEBHOOK_URL` で、CD の Terraform が New Relic に入れる。アプリのエージェントが送ったものだけを見る。
- Cloud Monitoring のアラート（メール）: 外形監視と、CD 以外による管理の操作。宛先は GitHub の Environment の secret `PLATFORM_ALERT_EMAIL` で、CD の Terraform が通知先に入れる。アプリ、New Relic、Discord の webhook のどれが止まっても届く。リポジトリも Actions のログも公開なので、宛先は tfvars や Environment の variable に置かない。
- 予算（メール）: billing account の管理者とユーザーに届く。`terraform/manual/billing/` を人が手で apply する。
- Readiness Monitor（`.github/workflows/readiness-monitor.yml`、Discord）: 同じ `DISCORD_ALERT_WEBHOOK_URL` に送る。外形監視の補助。
- CI とデプロイの失敗（Discord）: 各 workflow が `notify-discord.yml` を呼び、リポジトリの secret `DISCORD_WEBHOOK_URL` に送る。

## New Relic

しきい値は `terraform/services/monitoring/variables.tf` の既定値。どれも 5 分の窓で数える。

| 条件 | 重さ | 鳴る条件 | まず見るもの |
| --- | --- | --- | --- |
| API 5xx error rate | critical | API の transaction の 5% を超えて 5xx | [最初の 5 分](runbooks/README.md#最初の-5-分)。直近のデプロイからなら[戻す](runbooks/README.md#戻す) |
| Worker 5xx error rate | critical | worker の transaction の 5% を超えて 5xx。worker から 15 分何も届かないときも鳴る（loss of signal） | 同上。worker の revision とログ（[デプロイ後にジョブが全部死ぬ](architecture/cloud-logging-runbook.md#43-worker-のデプロイ後ジョブが全部死ぬ)） |
| Job failure spike | critical | `JobFailed` が 5 件を超えた | 失敗したジョブのログ（[ジョブが failed と表示されている](architecture/cloud-logging-runbook.md#41-ジョブが-failed-と表示されている)） |
| Stuck job spike | critical | QUEUED か RUNNING のまま止まったジョブ（`JobStuck`）が 1 件でもある | queue が止まっていないか（[止める](runbooks/README.md#ジョブの処理と-llm-の呼び出し)）、worker のログ |
| Billing/Stripe error spike | critical | API の `billing.` で始まるエラーが 3 件を超えた | API のログと、Stripe のダッシュボードの webhook |
| API p95 response time | warning | API の p95 が 3 秒を超えた | New Relic の遅い transaction |
| API auth error spike | warning | API の 401 と 403 が 100 件を超えた | 直近のデプロイ（認証の設定）。乱用なら[止める](runbooks/README.md#止める) |
| Worker p95 response time | warning | worker の p95 が 10 秒を超えた | worker のログと LLM の遅れ |
| Job notification failure spike | warning | `NotificationFailed`（ジョブの状態を Firestore に書けなかった）が 3 件を超えた | worker のログ |
| LLM rate-limit/quota pressure | warning | `LLMThrottled`（reason が `rate_limited`）が 30 件を超えた | Gemini の quota。続くなら queue を[止める](runbooks/README.md#ジョブの処理と-llm-の呼び出し) |
| Browser JS error spike | warning | 画面の JavaScript のエラーが 10 件を超えた | New Relic Browser と、直近の Deploy Frontend |
| Upload rejection spike | warning | `UploadRejected`（容量、大きさ、形式で断ったアップロード）が 50 件を超えた | API のログ。乱用なら[止める](runbooks/README.md#止める) |

## Cloud Monitoring（メール）

`terraform/services/platform_alerts/main.tf`。環境ごとに、その環境のプロジェクトに作る。開いたときと閉じたとき（回復から 30 分）にメールが届き、本文に手順書のリンクが付く。

| 条件 | 重さ | 鳴る条件 | まず見るもの |
| --- | --- | --- | --- |
| web unreachable | critical | `web_base_url` の `/` の外形監視（5 分ごと、アジア・ヨーロッパ・米国の 3 地点）が、2 地点以上で 10 分続けて失敗した。200 でも `<title>Synthify</title>` が無ければ失敗 | [uptime](runbooks/uptime.md) |
| api unreachable | critical | API の `/health?ready=1` の外形監視（同じ間隔と地点）。`X-Synthify-Readiness-Key` に `READINESS_MONITOR_KEY` を付ける。DB に届かなければ 503、鍵が違えば 401 で失敗 | [uptime](runbooks/uptime.md) |
| admin change outside CD | critical | CD のデプロイ用 SA と Google の主体以外が、IAM のポリシー、SA の鍵、Cloud Run のサービスとジョブ、Secret Manager の secret と版、Workload Identity のプロバイダを変えた（Admin Activity の監査ログ。1 件で） | [cloud-admin-changes](runbooks/cloud-admin-changes.md) |

## 予算（メール）

`terraform/manual/billing/`。CD では apply しない。

| 予算 | 鳴る条件 | まず見るもの |
| --- | --- | --- |
| `synthify-stage-budget`、`synthify-prod-budget` | その月の費用が環境ごとの予算（1,000 円）の 50%・90%・100% を超えた。月末の予測が 100% を超えた | [budget](runbooks/budget.md) |

## GitHub Actions

| 通知 | 鳴る条件 | 定義 | まず見るもの |
| --- | --- | --- | --- |
| Readiness check failed: `<env>` | API の `/health?ready=1`（DB まで見る）が、60 秒あけた 2 回とも失敗した（stage と prod） | `.github/workflows/readiness-monitor.yml` | [最初の 5 分](runbooks/README.md#最初の-5-分) |
| `<workflow>` / `<job>` failed | Backend、Web（PR と main）、Deploy Backend、Deploy Frontend（stage と prod）のジョブが落ちた | `.github/workflows/notify-discord.yml` を各 workflow が呼ぶ | 実行のログ。デプロイのスモークテストで落ちたときも、新しい版は出たまま（自動では戻らない） |

E2E External Integration（`e2e-nightly.yml`、毎日 stage）は、落ちても Discord には送らない。

## 知っておくこと

- Readiness Monitor は 15 分ごとの設定だが、GitHub の schedule は遅れる。直近 30 回（2026-11-13〜21）の実際の間隔は 3.7〜9.7 時間（中央 6.8 時間）だった。API と画面を外から見続けるのは Cloud Monitoring の外形監視で、Readiness Monitor は Discord にも届く補助として残している。60 日コミットが無いと、GitHub が schedule を止める。
- Discord の webhook が無効になると、New Relic、Readiness Monitor、CI の通知は届かない。Cloud Monitoring と予算のメールは届く。
- `PLATFORM_ALERT_EMAIL` が空でも、Cloud Monitoring のアラートは作られる。通知先が無いので、コンソールのインシデントにしか出ない。デプロイのログに warning が出る。
- New Relic のアラートは、New Relic にデータが届いているときにだけ鳴る。届かないことで鳴るのは worker の loss of signal だけ。API が起動できないときは、外形監視が鳴る。
- 画面（Firebase Hosting）は、外形監視が stage と prod を 5 分ごとに見る。動くかまでは、デプロイ直後のスモークテストと、stage に向けた毎日の E2E だけが見る。
- 手順書へのリンクが付くのは Cloud Monitoring のアラートだけ。New Relic と GitHub の通知には付かない。条件を足したら、この表も足す。

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
