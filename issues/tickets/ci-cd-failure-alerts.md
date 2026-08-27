# CI/CD失敗通知の動作確認と運用ドキュメント整備

## 背景

CI/CD失敗をDiscordへ通知する仕組みは実装済み。reusable workflow (`.github/workflows/notify-discord.yml`) が用意され、以下のワークフローから `if: failure()` で呼ばれている。

- `web.yml` (`needs: [checks, binaries, e2e]`)
- `backend.yml` (`needs: [build, terraform]`)
- `deploy-frontend.yml` (`needs: deploy`)
- `deploy-backend.yml` (`needs: deploy`)

通知内容はワークフロー名・job名・ブランチ/PR番号・commit SHA・actor・Actions実行画面へのリンクを含む。webhook URLは `DISCORD_WEBHOOK_URL` として登録済み。

残っているのは「本当に届くかの検証」と「届いた後に誰が何をするかの記述」の2点。通知が飛ばない設定ミスは、実際に失敗させるまで気づけない。

なお本番稼働中のアプリケーション異常については [application-monitoring-alerts.md](./application-monitoring-alerts.md) が別途扱う(通知先も `DISCORD_ALERT_WEBHOOK_URL` で分離されている)。

## 残タスク

### 1. 通知が実際に届くことを確認する

- [ ] 意図的にテストを失敗させたPRを立て、Discordに通知が届くことを確認する
- [ ] 通知メッセージから失敗箇所(どのワークフロー/どのjob/どのcommit)が一目で分かることを確認する
- [ ] 成功時には通知が飛ばないことを確認する(`if: failure()` の設定ミスがないか)
- [ ] 4ワークフローすべてで確認する。特に `deploy-backend.yml` / `deploy-frontend.yml` は
      `needs: deploy` のみのため、deploy job より前段で失敗した場合に通知されるかを確認する

### 2. アラート運用ドキュメントを整備する

現状、通知が届いた後の扱いがどこにも書かれていない。[docs/architecture/cloud-logging-runbook.md](../../docs/architecture/cloud-logging-runbook.md) にログ調査手順はあるが、CI/CD失敗通知からの導線がない。

- [ ] 通知を受けたら誰が対応するかを明記する
- [ ] 一次対応の手順(Actions実行画面の確認 → 再実行するか調査するかの判断基準)を書く
- [ ] main が壊れている場合のエスカレーション/revert方針を書く
- [ ] flaky test で通知が繰り返し飛ぶ場合の扱いを決める(通知疲れを防ぐ)
- [ ] 置き場所は README か CONTRIBUTING か runbook 配下かを決めて追記する

## 完了条件

- [ ] 4ワークフローすべてで失敗時にDiscord通知が届くことを確認済み
- [ ] 成功時に誤通知が飛ばないことを確認済み
- [ ] 通知を受けた人が次に何をすべきかがドキュメントから辿れる
