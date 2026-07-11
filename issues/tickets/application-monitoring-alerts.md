# アプリケーション監視アラートを導入する

## 背景

New Relic(APM: バックエンド、Browser: フロントエンド)によるログ・エラー・メトリクス収集基盤は整っているが、そこから先のアラートポリシー・通知チャネルは一切定義されていない。本番障害が発生しても誰も気づけない状態。

[ci-cd-failure-alerts.md](./ci-cd-failure-alerts.md) はCI/CD失敗の通知が対象で、本チケットは本番稼働中のアプリケーション異常を対象とする。

## 通知先

- CI/CD通知とは別のDiscordチャンネル・別webhook(`DISCORD_ALERT_WEBHOOK_URL`)を用意する。登録済み
- New RelicのWebhook通知先をDiscordのSlack互換エンドポイント(`.../slack`)に向けて送信する(`newrelic_notification_destination` type=WEBHOOK, url末尾に`/slack`を付与)

## アラート条件

| # | 条件 | データソース | 閾値(初期案) | Severity |
|---|---|---|---|---|
| 1 | 5xxエラー率 | NR APM (nrconnectインターセプタ / httpmiddlewareのstatusログ) | 5分間で5xx率が5%超 | Critical |
| 2 | readiness/DB接続失敗 | GitHub Actions cronから `/health?ready=1` を定期監視(NR Syntheticsは使わない。コスト不透明なため見送り) | 2回連続失敗 | Critical |
| 3 | レスポンスタイム増大 | NR APM transaction duration (`http.request.slow` ログ相当) | p95が3秒超(5分間) | Warning |
| 4 | フロントJSエラー急増 | NR Browser jserrorsイベント | 直近5分でbaseline比3倍 or 絶対数10件超 | Warning |

Alert PolicyはCritical/Warningで2つに分ける(Discord側で緊急度を区別できるようにするため)。

## 実装方針

### 1. readiness監視(GitHub Actions cron)

- CI/CD失敗通知([ci-cd-failure-alerts.md](./ci-cd-failure-alerts.md))のreusable workflow (`notify-discord.yml`) を再利用
- `schedule` トリガーで数分おきに `/health?ready=1` を叩き、失敗(2回連続)したらDiscordへ通知
- 追加コストはGitHub Actions実行時間のみ(Actions minutes枠内)

### 2. NR Alert Policy(Terraform管理)

実装済み構成(`terraform/services/*` は既存の全サービスが並ぶ場所なので、`modules/newrelic_alerts` ではなく `services/monitoring` として追加した):

```
terraform/
  environments/
    main.tf           # provider "newrelic" {}, module "monitoring" 呼び出し
    variables.tf       # new_relic_account_id / new_relic_api_key / new_relic_browser_app_name / discord_alert_webhook_url
    versions.tf         # required_providers に newrelic/newrelic ~> 3.0 を追加
  services/
    monitoring/
      main.tf            # newrelic_alert_policy (critical / warning) + newrelic_nrql_alert_condition x3
      notification.tf      # newrelic_notification_destination/channel (Discord Slack互換webhook), newrelic_workflow x2
      variables.tf
      versions.tf          # 子モジュール側にも required_providers が必須(暗黙解決だと hashicorp/newrelic を探しにいって失敗する)
```

- `newrelic` Terraform providerを追加、`terraform fmt`/`terraform validate` 済み
- 条件は`newrelic_nrql_alert_condition`で統一。ただしreadiness/DB接続失敗はNRQLではなく別経路(下記)
- New Relic User Key/Account IDは発行済み、GitHub Environment Secretsに登録済み(stage/prod両方)

## GitHub Secrets/Variables(登録済み、stage/prod両Environment)

- Secrets: `NEW_RELIC_ACCOUNT_ID`, `NEW_RELIC_API_KEY`, `DISCORD_ALERT_WEBHOOK_URL`, `READINESS_MONITOR_KEY`(新規生成)
- Variables: `NEW_RELIC_BROWSER_APP_NAME`
- `deploy-backend.yml` の Pass 2 / Pass 3 (`terraform apply`) に `-var` として配線済み

## readiness監視の実装(GitHub Actions cron)

当初案は既存の`readiness_api_key`(デプロイ毎に乱数生成)を流用する想定だったが、それだと外部からの定期監視に使える固定キーが存在しないことが判明。そのため恒久的な監視専用キー`readiness_monitor_key`/`SYNTHIFY_READINESS_MONITOR_KEY`を新設した:

- `apps/api/internal/config/config.go`: `ReadinessMonitorKey`を追加
- `apps/api/cmd/server/main.go`: `healthHandler`が`readinessKey`と`readinessMonitorKey`のどちらでも認証を通すように変更(`readinessAuthorized`を2回呼ぶ)
- `terraform/services/api/variables.tf` / `main.tf`: `readiness_monitor_key`をsensitive env varとしてCloud Runに配線
- `.github/workflows/readiness-monitor.yml`: 新規。5分おきに`stage`/`prod`両方の`/health?ready=1`をmatrix jobで叩き、30秒空けて1回リトライ、2回とも失敗したら`DISCORD_ALERT_WEBHOOK_URL`へ通知。`notify-discord.yml`とは別実装(commit/PRの概念がなく、定期実行なので専用の軽量な通知ステップにした)

## 実装タスク

- [x] Discord Webhook URLを発行(`DISCORD_ALERT_WEBHOOK_URL`としてSecrets登録)
- [x] New Relic User Key(Terraform用)を発行・登録
- [x] `terraform/services/monitoring/` モジュールを作成(Critical/Warning 2ポリシー、NRQL条件3種)
- [x] `terraform/environments/main.tf` からモジュールを呼び出し、`deploy-backend.yml`に変数を配線
- [x] readiness監視用の恒久キーを新設し、バックエンド(Go)・terraform・GitHub Secretsに配線
- [x] readiness監視用のGitHub Actions cron workflow (`readiness-monitor.yml`) を作成
- [ ] stageへterraform applyし、New RelicのWebhook通知がDiscord Slack互換エンドポイントで実際に動作するか実機検証
- [ ] 各アラートを意図的に発火させて通知が届くことを確認する(ステージング環境で検証)
- [ ] 閾値のチューニング(初期案は仮値のため、本番トラフィックを見て調整)
- [ ] 運用ドキュメント追記(アラート受信時の一次対応手順)

## 完了条件

- [ ] 5xxエラー率上昇時にCritical通知が届く
- [ ] readiness失敗時にCritical通知が届く
- [ ] レスポンスタイム増大時にWarning通知が届く
- [ ] フロントJSエラー急増時にWarning通知が届く
- [ ] 誤検知(閾値が過敏すぎる)がないか、導入後1週間程度様子を見て調整済み
- [ ] 運用ドキュメントに一次対応手順が記載されている

## 未確定事項 / 要検証

- New Relic WebhookをDiscordのSlack互換エンドポイントに送った際、embed表示が崩れないか(崩れる場合は中継サービスの検討が必要)
- 各閾値は仮案。本番トラフィックのベースラインが分かってから調整する
