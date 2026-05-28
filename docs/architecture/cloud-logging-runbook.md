# Cloud Logging Runbook

Synthify の stage / prod は Cloud Run 上に居て、worker と api は slog の JSON を stdout に流す。Cloud Logging から原因を引き出すための定型クエリと、それを楽にする `make` ショートカットの使い方をまとめる。

関連:
- ショートカット定義: [Makefile](../../Makefile) の `logs-*` ターゲット
- 構造化ログを出す interceptor: [internal/platform/observability/connect_mask.go](../../internal/platform/observability/connect_mask.go)
- 失敗ジョブが NR にも流れる経緯: [../../docs/learn/upload-pipeline-hardening.md](../learn/upload-pipeline-hardening.md) と [apps/worker/pkg/worker/job/lifecycle/service.go](../../apps/worker/pkg/worker/job/lifecycle/service.go)

---

## 1. 何が Cloud Logging に出るか

ログは大まかに 3 系統に分かれる。**どの系統を見るか間違えると "何も出てない" 状態になる** ので最初に押さえる。

| ログ種別 | logName | 何が入るか |
|---|---|---|
| アプリ stdout (slog JSON) | `run.googleapis.com/stdout` | worker / api 自身が出した構造化ログ。`jsonPayload.msg` / `jsonPayload.error` / `jsonPayload.job_id` 等のキー |
| Cloud Run リクエスト | `run.googleapis.com/requests` | HTTP メタデータのみ (method / path / status / latency)。body や error は出ない |
| Cloud Run システム | `run.googleapis.com/varlog/system` | コンテナ起動・終了・GCSFuse 等の出力 |

**ハマりやすい挙動**:
- `severity>=ERROR` で絞ると `requests` の status=500 行は拾えるが、stdout の slog は `level=ERROR` が `severity` にマップされないことがあり漏れる。stdout も合わせて見たい時は severity フィルタを外して timestamp 窓で絞る。
- `resource.labels.service_name` が正しい (例: `synthify-worker-stage`)。`labels.service_name` ではない。
- `--format='value(...)'` で複数フィールド指定したいときは tab 区切りで並ぶ。中身が空の field があると見落とすので、迷ったら `--format=json` で全部見る。

---

## 2. 構造化ログのキー (Synthify 固有)

worker / api が必ず付けるキー:

| キー | 意味 | フィルタ例 |
|---|---|---|
| `jsonPayload.msg` | イベント名。例: `worker.execute_plan_received`, `rpc.internal_error`, `job.dispatch_failed` | `jsonPayload.msg="job.dispatch_failed"` |
| `jsonPayload.error` | エラー文字列 (slog の `"error"` attribute) | `jsonPayload.error:"NOT_FOUND"` (部分一致) |
| `jsonPayload.job_id` | document processing job の ULID | `jsonPayload.job_id="01KSPNQR670W96MNC0J8ST250K"` |
| `jsonPayload.workspace_id` / `document_id` / `tree_id` | 関連エンティティの ID | 同上 |
| `jsonPayload.procedure` | Connect 経由の RPC パス。`MaskInternalErrorsHandlerOptions` が出す | `jsonPayload.procedure:"ExecuteApprovedPlan"` |
| `trace` (top-level) | Cloud Trace ID。同一リクエスト内の全ログを横断できる | `trace="projects/<project>/traces/<hex>"` |

---

## 3. `make` ショートカット

すべて [Makefile](../../Makefile) で定義。共通の override:

- `SINCE='10 minutes ago'` — 検索窓 (default `'1 hour ago'`、`date -d` の任意形式)
- `LIMIT=50` — 件数 (default 30)
- `STAGE_PROJECT=...` / `PROD_PROJECT=...` — project id を差し替え

### サービス単位で最近の ERROR を見る

```bash
# stage worker の直近 1 時間
make logs-stage-worker

# stage api の直近 30 分を 50 件
make logs-stage-api SINCE='30 minutes ago' LIMIT=50

# prod (PROD_PROJECT は要 override)
make logs-prod-worker PROD_PROJECT=<prod-project-id>
```

返ってくる列: `timestamp service severity msg error`

### job_id で api + worker 横断

詰まったジョブの一次切り分けはこれ。

```bash
make logs-stage-job JOB_ID=01KSPNQR670W96MNC0J8ST250K SINCE='3 days ago'
```

`asc` 順 (古い → 新しい) で並ぶので「どの service のどの stage で死んだか」が時系列で見える。

`MaskInternalErrorsHandlerOptions` のおかげで、handler が `CodeInternal` で返したエラーは原文が `rpc.internal_error` イベントで stdout に残る。クライアントには汎用文 ("internal server error") に差し替わって渡るが、Cloud Logging では原文が見える。

### trace_id で同一リクエスト内

`make logs-*-worker` で拾ったエラー行に `trace="projects/.../traces/<hex>"` フィールドがある。これを使って前後を含めた完全な系列を引く:

```bash
make logs-stage-trace TRACE_ID=7e147babdc8a3889202f157a74706b44
```

---

## 4. 典型的な障害パターンとクエリ

### 4.1 「ジョブが failed と表示されている」

1. Firestore か Synthify Web から `jobId` を控える。
2. `make logs-stage-job JOB_ID=<ulid> SINCE='1 hour ago'`
3. `worker.execute_plan_received` 以降のエラーを順に見る。`api` 側の `worker.dispatcher_rpc_failed` と `worker` 側の `rpc.internal_error` が同じエラーをそれぞれ吐いているはずなので、どちらか片方を見れば理由は判明する。

### 4.2 「特定の API リクエストが 500」

1. ブラウザ devtools で Network → 失敗したリクエストの response header に `x-cloud-trace-context` があれば trace id を取れる。
2. `make logs-stage-trace TRACE_ID=<hex>`

trace id が無いなら `make logs-stage-api SINCE='5 minutes ago'` で時系列に並べて該当 procedure を探す。

### 4.3 「worker のデプロイ後ジョブが全部死ぬ」

リビジョン全体の問題が疑われる。

```bash
make logs-stage-worker SINCE='10 minutes ago' LIMIT=100
```

`rpc.internal_error` の `error` 列を眺めれば、Vertex AI 404 / GCS 権限 / DB 接続のようなインフラ起因はすぐ分かる。

---

## 5. 生 gcloud を使うとき

`make` ショートカットでカバーしきれない調査は直接 gcloud を叩く。覚えるべきは 3 つだけ:

```bash
# severity と service だけで絞る
gcloud logging read \
  'resource.labels.service_name="synthify-worker-stage" AND severity>=ERROR' \
  --project=<project-id> --limit=20 --order=desc --format=json

# 特定 jsonPayload key で絞る
gcloud logging read \
  'jsonPayload.workspace_id="01KSPN..."' \
  --project=<project-id> --limit=50 --format=json --order=asc

# trace で絞る
gcloud logging read \
  'trace="projects/<project-id>/traces/<hex>"' \
  --project=<project-id> --limit=50 --format=json --order=asc
```

`--format=json` を最初に出すのが鉄則。`value(...)` で絞ったあとに足りないと気づくと二度手間になる。

## 6. NR との使い分け

両方を併用する前提で組まれている。

| 用途 | 見る場所 |
|---|---|
| 障害発生時の一次切り分け (どこで何が落ちたか) | Cloud Logging (この runbook) |
| stack trace を集約 / 関連 transaction を辿る | NR の Errors inbox / APM |
| `JobFailed` / `UploadRejected` の傾向分析 (account 別, reason 別) | NR の NRQL (`SELECT count(*) FROM JobFailed FACET reason TIMESERIES`) |

Cloud Logging には原文の生ログがある。NR にはアプリ側で明示的に送った CustomEvent と、Connect interceptor 経由で自動収集された transaction / error がある。両者は冗長だが、見たい切り口が違う。
