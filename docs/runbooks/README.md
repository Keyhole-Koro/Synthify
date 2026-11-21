# 手順書（runbook）

障害やアラートのときの手順。アラートの一覧と鳴る条件は[アラート](../alerts.md)にある。ログの引き方は [Cloud Logging Runbook](../architecture/cloud-logging-runbook.md)、デプロイ直後の確認は [Stage / Prod Smoke Test](../architecture/stage-prod-smoke-tests.md) にある。

ここに書いたコマンドのうち、クラウドの資源を変えるもの（戻す、止める、秘密を差し替える）はオーナーが流す。エージェントは手順とコマンドを示すところまで。

## 環境と名前

| | stage | prod |
| --- | --- | --- |
| デプロイのきっかけ | `stage` ブランチへの push | `prod` ブランチへの push |
| GCP のプロジェクトとリージョン | `terraform/tfvars/stage.tfvars` | `terraform/tfvars/prod.tfvars` |
| 画面（Firebase Hosting） | https://stage.synthify.keyhole.work | https://synthify.keyhole.work |
| New Relic の APM | `synthify-api-stage`、`synthify-worker-stage` | `synthify-api`、`synthify-worker` |

| もの | 名前（`<env>` は `stage` か `prod`） |
| --- | --- |
| API | Cloud Run の `synthify-api-<env>` |
| worker | Cloud Run の `synthify-worker-<env>`。API が Cloud Tasks に積んだジョブを処理する |
| ジョブの queue | Cloud Tasks の `synthify-pipeline-<env>` |
| monitor（管理画面） | Cloud Run の `synthify-monitor-<env>` |
| LLM の評価 | Cloud Run Job の `synthify-eval-<env>` と、それを毎日起動する Cloud Scheduler の `synthify-eval-<env>-schedule` |
| DB | CockroachDB Serverless（GCP の外）。接続文字列は Secret Manager の `synthify-database-dsn` |

下のコマンドは、リポジトリのルートで次を済ませてから流す。

```bash
ENV=prod   # stage のときは stage
PROJECT="$(bash scripts/tfvar.sh "$ENV" project_id)"
REGION="$(bash scripts/tfvar.sh "$ENV" region)"
```

## 最初の 5 分

1. Discord の通知で、どのアラートが、どの環境で鳴ったかを見る。New Relic からの通知には issue の URL が付く。それぞれの意味は[アラート](../alerts.md)。
2. 直近にデプロイしたかを見る。

   ```bash
   gh run list --workflow deploy-backend.yml --limit 5
   gh run list --workflow deploy-frontend.yml --limit 5
   gcloud run revisions list --service "synthify-api-$ENV" --project "$PROJECT" --region "$REGION" --limit 5
   ```

3. 外から動いているかを見る。Cloud Monitoring の「Uptime checks」に、画面と API の `/health?ready=1` を 5 分ごとに 3 地点から見た結果がある（[uptime](uptime.md)）。`/health` は DB に触らずに 200 を返す。DB まで見る `/health?ready=1` は鍵が要るので、手で見るなら Readiness Monitor を流す（`gh workflow run readiness-monitor.yml`。stage と prod の両方を見る）。

   ```bash
   API_URL="$(gcloud run services describe "synthify-api-$ENV" --project "$PROJECT" --region "$REGION" --format 'value(status.url)')"
   curl -s -o /dev/null -w '%{http_code}\n' "$API_URL/health"
   curl -s -o /dev/null -w '%{http_code}\n' "$(bash scripts/tfvar.sh "$ENV" web_base_url)"
   ```

4. エラーを見る。
   - Cloud Logging: `make logs-$ENV-api PROD_PROJECT="$PROJECT"`、`make logs-$ENV-worker PROD_PROJECT="$PROJECT"`。アプリのログに加えて、起動の失敗やメモリ不足のような Cloud Run 自体のエラーも出る。Makefile の `PROD_PROJECT` の既定値は prod のプロジェクト ID と違うので、prod では必ず渡す。ジョブごと、trace ごとの引き方は [Cloud Logging Runbook](../architecture/cloud-logging-runbook.md#4-典型的な障害パターンとクエリ)。
   - New Relic: APM（上の表の名前）のエラーと遅い transaction、Browser の JavaScript のエラー。
5. 決める。
   - 直近のデプロイから始まったなら、[戻す](#戻す)。
   - 原因が分からず利用者への影響が大きいとき、または LLM の費用が増え続けているときは、影響のある所を[止める](#止める)。
   - 秘密が漏れたなら、[秘密が漏れたとき](#秘密が漏れたとき)。

## 戻す

前に prod に出ていたコミットを、もう一度デプロイする。prod ブランチは main のコミットを push して進めている（今の prod は main の祖先）。prod ブランチを前のコミットに戻して push すると、Deploy Backend と Deploy Frontend がそのコミットで走る。最近の Deploy Backend は 1 回 7〜13 分かかっている。stage も同じ手順で戻せる（`prod` を `stage` に読み替える）。

1. 前に出ていたコミットを探す。prod で成功した Deploy Backend のうち、壊れた版より前で最後のもの。

   ```bash
   gh run list --workflow deploy-backend.yml --branch prod --status success --limit 5 \
     --json headSha,createdAt,displayTitle --jq '.[] | "\(.headSha)  \(.createdAt)  \(.displayTitle)"'
   ```

2. 戻す範囲に、DB の migration、Terraform、Firestore の rules の変更が無いかを見る。

   ```bash
   PREV=<前に出ていたコミット>
   git fetch origin prod
   git diff --stat "$PREV" origin/prod -- db/migrations terraform firestore.rules
   ```

   - `db/migrations` に差分があるときは、この方法では戻せない。前のコミットには DB の今の版の migration が無いので、デプロイは Apply DB migrations で `no migration found for version N` と言って止まる。その前の Terraform apply (platform only) は、前のコミットの設定のまま走ってしまう。直すコミットを main に入れ、それを prod に出す（前に進めて直す）。
   - `terraform` に差分があるときは、前のコミットの Terraform が apply される。その後に足したリソースは消え、変えた設定は前に戻る。差分を読み、消えて困るもの（データのあるバケット、使っている secret など）が無いことを確かめてから戻す。
   - `firestore.rules` に差分があるときは、前の rules に戻る。

3. prod を前のコミットに戻す。push すると両方のデプロイが走る。prod ブランチには今は保護が無いので、force push できる。

   ```bash
   git push --force-with-lease origin "$PREV:prod"
   ```

4. Deploy Backend と Deploy Frontend が成功し、スモークテストと Readiness Monitor が通ることを見る。デプロイは、そのときの GitHub の Environment の変数と secret を使う（コミットと一緒には戻らない）。
5. 直した版を出すときは、いつもどおり main のコミットを prod に push する。前のコミットは main の祖先なので、そのまま進められる。

## 止める

止める範囲ごとに分けている。上から順に、影響が小さい。

### ジョブの処理と LLM の呼び出し

ドキュメントの処理（worker での LLM の呼び出し）は、Cloud Tasks の queue から worker に渡る。queue を止めると、新しいジョブは積まれるだけで処理されない。動いているジョブは最後まで動く。再開すると、積まれていたものが流れる。LLM の評価は Cloud Scheduler が毎日起動するので、これも止める。

```bash
gcloud tasks queues pause "synthify-pipeline-$ENV" --project "$PROJECT" --location "$REGION"
gcloud scheduler jobs pause "synthify-eval-$ENV-schedule" --project "$PROJECT" --location "$REGION"

# 再開する
gcloud tasks queues resume "synthify-pipeline-$ENV" --project "$PROJECT" --location "$REGION"
gcloud scheduler jobs resume "synthify-eval-$ENV-schedule" --project "$PROJECT" --location "$REGION"
```

- Terraform はこの 2 つの止まっている状態を管理しないので、デプロイしても再開しない。手で再開する。
- 止めている間は、待っているジョブで Stuck job spike が鳴ることがある。

### API

API は誰でも呼べる設定（`allUsers` に `roles/run.invoker`。Terraform の `allow_unauthenticated`）になっている。これを外すと、ブラウザからの呼び出しも、Stripe の webhook も、worker から API への呼び出しも、Cloud Run が 403 で断る。先に上の queue を止めておく。止めないと、処理中のジョブが API を呼べずに失敗する。

```bash
gcloud run services remove-iam-policy-binding "synthify-api-$ENV" --project "$PROJECT" --region "$REGION" \
  --member=allUsers --role=roles/run.invoker

# 戻す（次のデプロイでも Terraform が付け直す）
gcloud run services add-iam-policy-binding "synthify-api-$ENV" --project "$PROJECT" --region "$REGION" \
  --member=allUsers --role=roles/run.invoker
```

止めている間は、Readiness Monitor、New Relic、Cloud Monitoring の外形監視（api unreachable）のアラートが鳴る。

### 画面

```bash
bunx firebase-tools@13 hosting:disable --project "$PROJECT" --force
```

Firebase Hosting がサイトを出さなくなる（独自ドメインも）。戻すには、もう一度デプロイする（Deploy Frontend を流す。prod なら `gh workflow run deploy-frontend.yml --ref prod -f environment=prod`）。

### 一部のアカウントだけに絞る

API には許可リストがある（環境変数 `SYNTHIFY_ALLOWED_USER_EMAILS`。空なら制限なし）。stage はこれで 1 アカウントに絞っている。prod でも、`terraform/tfvars/prod.tfvars` に `allowed_user_emails`（メールアドレスをカンマ区切り）を書いてデプロイすると、それ以外のアカウントの操作は PermissionDenied になる。デプロイが要るのですぐには効かない。リポジトリは public なので、書いたアドレスは公開される。

## 秘密が漏れたとき

1. 漏れた値を、発行元で無効にして作り直す。リポジトリや履歴から消しても、public なので漏れは取り消せない。
2. 新しい値を入れる。

   | 秘密 | 置き場所 | 使う所 | 作り直す所 |
   | --- | --- | --- | --- |
   | `synthify-database-dsn` | Secret Manager | API、worker、eval、CD の migration | CockroachDB Cloud（SQL ユーザーのパスワード） |
   | `synthify-monitor-database-dsn` | Secret Manager | monitor（読み取り専用） | CockroachDB Cloud（monitor のユーザー） |
   | `synthify-stripe-secret-key` | Secret Manager | API | Stripe のダッシュボード |
   | `synthify-stripe-webhook-secret` | Secret Manager | API（webhook の署名の検証） | Stripe のダッシュボード（webhook の endpoint） |
   | `synthify-gemini-api-key` | Secret Manager | worker、eval | 鍵を発行した所 |
   | `synthify-new-relic-license-key` | Secret Manager | API、worker | New Relic |
   | `synthify-internal-worker-token` | Secret Manager | worker から API への呼び出し | 自分で作る（`openssl rand -base64 32`） |
   | `READINESS_MONITOR_KEY` | GitHub の Environment（stage・prod）の secret | Readiness Monitor。CD が API と、Cloud Monitoring の外形監視にも渡す | 自分で作る |
   | `DISCORD_ALERT_WEBHOOK_URL` | GitHub の Environment の secret | Readiness Monitor。CD が New Relic の通知先に入れる | Discord |
   | `NEW_RELIC_API_KEY` | GitHub の Environment の secret | CD の Terraform（New Relic のアラートを作る） | New Relic |
   | `DISCORD_WEBHOOK_URL` | GitHub のリポジトリの secret | CI とデプロイの失敗の通知 | Discord |

   Secret Manager の値は、新しい版として足す。値はコマンドの引数に書かず、標準入力から渡す（貼って Ctrl-D）。

   ```bash
   gcloud secrets versions add "synthify-<名前>" --project "$PROJECT" --data-file=-
   ```

   GitHub の secret は、リポジトリの Settings の Environments（または Secrets and variables）で書き換える。

3. デプロイし直して、新しい値を読ませる。Deploy Backend は毎回 API と worker の新しい revision を作り（readiness の鍵を毎回作り直すため）、新しい revision は Secret Manager の最新の版を読む。`READINESS_MONITOR_KEY` と `DISCORD_ALERT_WEBHOOK_URL` も CD が API、外形監視、New Relic に入れ直す（鍵が変わると、外形監視は作り直される）。monitor と eval は、次にインスタンスが起動したときに最新の版を読む。

   ```bash
   gh workflow run deploy-backend.yml --ref "$ENV" -f environment="$ENV"
   ```

4. 古い版を無効にする。

   ```bash
   gcloud secrets versions list "synthify-<名前>" --project "$PROJECT"
   gcloud secrets versions disable <古い版の番号> --secret "synthify-<名前>" --project "$PROJECT"
   ```

5. 漏れていた間に使われていないかを、発行元の記録（Stripe のイベント、Gemini の利用量、CockroachDB の接続など）で見る。

CD は GCP に Workload Identity Federation で入る。`GCP_WIF_PROVIDER` と `GCP_WIF_SA_EMAIL` は鍵ではなく名前なので、漏れても作り直すものは無い。`NEXT_PUBLIC_` で始まる値（Firebase の web の設定、New Relic Browser の鍵）は配る画面に入るもので、秘密ではない。
