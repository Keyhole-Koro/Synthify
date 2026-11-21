# アラート

何が起きたら、どこに知らせるかの一覧。鳴ったときの手順は[手順書](runbooks/README.md)にある。定義は Terraform（`terraform/services/monitoring/`）と `.github/workflows/` を正とする。

通知はどれも Discord に届く。経路は 3 つある。

- New Relic のアラート: 環境ごとのポリシー `synthify-critical-<env>` と `synthify-warning-<env>`。New Relic の workflow が、issue の題と URL を Discord の webhook に送る。webhook の URL は GitHub の Environment の secret `DISCORD_ALERT_WEBHOOK_URL` で、CD の Terraform が New Relic に入れる。
- Readiness Monitor（`.github/workflows/readiness-monitor.yml`）: 同じ `DISCORD_ALERT_WEBHOOK_URL` に送る。
- CI とデプロイの失敗: 各 workflow が `notify-discord.yml` を呼び、リポジトリの secret `DISCORD_WEBHOOK_URL` に送る。

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

## GitHub Actions

| 通知 | 鳴る条件 | 定義 | まず見るもの |
| --- | --- | --- | --- |
| Readiness check failed: `<env>` | API の `/health?ready=1`（DB まで見る）が、60 秒あけた 2 回とも失敗した（stage と prod） | `.github/workflows/readiness-monitor.yml` | [最初の 5 分](runbooks/README.md#最初の-5-分) |
| `<workflow>` / `<job>` failed | Backend、Web（PR と main）、Deploy Backend、Deploy Frontend（stage と prod）のジョブが落ちた | `.github/workflows/notify-discord.yml` を各 workflow が呼ぶ | 実行のログ。デプロイのスモークテストで落ちたときも、新しい版は出たまま（自動では戻らない） |

E2E External Integration（`e2e-nightly.yml`、毎日 stage）は、落ちても Discord には送らない。

## 知っておくこと

- Readiness Monitor は 15 分ごとの設定だが、GitHub の schedule は遅れる。直近 30 回（2026-11-13〜21）の実際の間隔は 3.7〜9.7 時間（中央 6.8 時間）だった。API が止まっても、半日近く気づかないことがある。
- 通知は Discord の webhook だけに届く。webhook が無効になると、どのアラートも届かない。
- New Relic のアラートは、New Relic にデータが届いているときにだけ鳴る。届かないことで鳴るのは worker の loss of signal だけ。
- 画面（Firebase Hosting）が落ちたことを見るのは、デプロイ直後のスモークテストと、stage に向けた毎日の E2E だけ。prod の画面を見続けるものは無い。
- 通知に手順書へのリンクは付かない。条件を足したら、この表も足す。
