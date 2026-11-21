# web / api unreachable

| | |
| --- | --- |
| 条件 | Cloud Monitoring の外形監視（5 分ごと、3 地点）が、2 地点以上で 10 分続けて失敗した |
| 監視先 | `web`: `web_base_url` の `/`（`<title>Synthify</title>` を含むこと）。`api`: API の Cloud Run の URL の `/health?ready=1`（本文が `ok` であること。`X-Synthify-Readiness-Key` に `READINESS_MONITOR_KEY` を付ける） |
| 通知先 | メール（GitHub の environment secret `PLATFORM_ALERT_EMAIL`） |
| 定義 | `terraform/services/platform_alerts/main.tf` |

## 意味

外から画面か API に届かない。`api` の readiness は DB（CockroachDB）にも触れるので、API は動いていても DB に届かなければ落ちる。夜間でトラフィックが無くても鳴る。

## 切り分け

失敗の理由は Cloud Monitoring の「Uptime checks」で、チェックを開くと地点ごとに見える。

| 落ちたもの | 疑う場所 |
| --- | --- |
| `web` だけ | Firebase Hosting、ドメイン（`keyhole.work` の DNS）、証明書 |
| `api` だけで 503 | DB。CockroachDB Cloud のコンソールで、クラスタが止まっていないか、上限に当たっていないか。`synthify-database-dsn` の値 |
| `api` だけで 401 | 外形監視の鍵と API の鍵が違う。GitHub の `READINESS_MONITOR_KEY` を変えたのに、まだデプロイしていないなど。デプロイすれば揃う |
| `api` だけで 5xx・タイムアウト | Cloud Run。起動に失敗していないか、直前のデプロイ |
| 両方 | ネットワーク、GCP や Firebase の障害 |

```bash
PROJECT=synthify-stage-491705   # prod は synthify-491705
ENV=stage                       # prod は prod
# いま流れている版
gcloud run services describe "synthify-api-$ENV" --project "$PROJECT" --region asia-northeast1 \
  --format 'value(status.traffic)'
# 直近のエラー（Makefile の logs-* でも引ける。docs/architecture/cloud-logging-runbook.md）
gcloud logging read "resource.labels.service_name=\"synthify-api-$ENV\" AND severity>=ERROR" \
  --project "$PROJECT" --freshness 1h --limit 30
```

## 対処

- 直前にデプロイしていれば、Cloud Run を前の Revision に戻す: `gcloud run services update-traffic "synthify-api-$ENV" --to-revisions <revision>=100 --project "$PROJECT" --region asia-northeast1`。次の CD の apply で Terraform の状態に戻るので、原因を直してから出し直す。
- DB なら、CockroachDB Cloud の状態を見る。
- GCP や Firebase の障害なら待つ。

## 閉じる条件

外形監視が通り、インシデントが閉じた（回復から 30 分で自動で閉じる）。
