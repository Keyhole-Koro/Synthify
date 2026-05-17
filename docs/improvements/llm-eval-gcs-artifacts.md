# LLM Eval Runner の GCS Artifact 保存

## 背景

現在の LLM Eval Runner は Cloud Run Job として自動実行できるが、結果は stdout に JSON report を出すだけである。
Cloud Logging には残るものの、以下の用途には弱い。

- 過去 run の report を安定した URI で参照したい
- CI/CD や dashboard から最新結果を取得したい
- golden diff / trend 集計 / prompt variant 比較の入力として report artifact を再利用したい
- Cloud Logging の検索に依存せず、eval の成果物を明示的に保管したい

そのため、Eval Runner の report を GCS に immutable artifact として保存する。

## 方針

最初の実装では **追記専用の run artifact** に絞る。

- `latest.json` のような上書きポインタは作らない
- GCS object は run ごとに一意な path に保存する
- eval runtime service account には `roles/storage.objectCreator` のみ付与する
- report schema は stdout / `--out` と同じ JSON bytes を使う
- Cloud Logging stdout は残す。GCS 保存は追加出力であり置き換えではない

## 保存先

Terraform から Eval Job に GCS prefix を渡す。

```text
EVAL_OUTPUT_GCS_URI=gs://{uploads_bucket}/eval/{environment}/runs
```

CLI は run ごとの object path を自動生成する。

```text
gs://{uploads_bucket}/eval/{environment}/runs/{yyyy}/{mm}/{dd}/{run_id}.json
```

`run_id` は UTC timestamp と短いランダム suffix を組み合わせる。

```text
20260517T042000Z-a1b2c3.json
```

同一秒に複数 run が起きても衝突しないことを目的に suffix を付ける。

## CLI 変更

`apps/eval/cmd` に flag を追加する。

```bash
--out-gcs gs://bucket/prefix
```

Cloud Run Job では env から渡すため、flag 未指定時は `EVAL_OUTPUT_GCS_URI` を見る。

優先順位:

1. `--out-gcs`
2. `EVAL_OUTPUT_GCS_URI`
3. 未設定なら GCS 保存しない

実行例:

```bash
go run ./apps/eval/cmd \
  --cases apps/eval/cases \
  --format json \
  --out-gcs gs://synthify-stage-491705-synthify-uploads-stage/eval/stage/runs
```

stdout と `--out` の挙動は変えない。GCS upload に失敗した場合は eval run 全体を失敗扱いにして exit 1 とする。

## 実装構成

### 既存 GCS 実装の流用判断

既存の GCS 関連実装は用途が異なるため、eval artifact 保存にはそのまま使わない。

| 既存実装 | 用途 | eval artifact への適性 |
| :--- | :--- | :--- |
| `packages/shared/storage.FileSystem` | GCS FUSE / local mount 経由で document、cache、checkpoint を読む/書く | 不採用。Cloud Run Job に GCS FUSE mount を追加する必要があり、artifact 追記保存には重い |
| `FileSystem.WriteCache` / `WriteCheckpoint` | mount path 配下に temp file + rename で JSON を保存 | 不採用。cache/checkpoint は worker 内部状態であり、eval run artifact の URI/権限/retention と責任が違う |
| `storage.BuildDocumentUploadURL` / `BuildDocumentSourceURL` | GCS JSON API / emulator 用 document URL を組み立てる | 不採用。document upload/source 専用で、service account による direct object write ではない |
| `app.NewGCSSignedDocumentUploadURLIssuer` | browser / client に署名付き PUT URL を渡す | 不採用。eval Job 自身が書くため署名 URL を発行する必要がない |
| `app.Bootstrap` | API/worker 用 store、upload URL issuer、notifier を組み立てる | 不採用。eval CLI は DB / notifier を必要としない |

採用する方式:

- eval runtime service account に `roles/storage.objectCreator` を付ける
- `apps/eval/artifact` が `cloud.google.com/go/storage` の writer で直接 object を作る
- GCS FUSE、署名 URL、DB store 初期化は使わない

理由:

- Cloud Run Job で追加 mount を持たずに動く
- IAM が単純で、必要権限を object 作成だけに絞れる
- stdout / `--out` と同じ report bytes をそのまま保存できる
- document upload pipeline と eval artifact pipeline の責任境界を混ぜない

追加 package:

```text
apps/eval/artifact
```

責任:

- `gs://` URI を parse する
- run artifact path を生成する
- JSON report bytes を GCS に upload する

想定 API:

```go
type GCSConfig struct {
    PrefixURI string
    Now       func() time.Time
    Rand      io.Reader
}

type UploadResult struct {
    URI string
}

func UploadGCS(ctx context.Context, cfg GCSConfig, report []byte) (UploadResult, error)
```

`cloud.google.com/go/storage` は既に root module の indirect dependency にあるため、新しい外部依存追加は不要の見込み。

## Terraform 変更

`terraform/services/eval`:

- env `EVAL_OUTPUT_GCS_URI` を追加
- 値は module input として受ける

`terraform/environments/main.tf`:

```hcl
eval_output_gcs_uri = "gs://${module.platform.uploads_bucket_name}/eval/${var.environment}/runs"
```

`terraform/services/platform`:

- eval runtime service account に uploads bucket への `roles/storage.objectCreator` を付与する
- bucket 全体への付与でよい。GCS IAM は prefix 単位に切れないため、アプリ側 path で `eval/{environment}/runs` に固定する

IAM:

```hcl
resource "google_storage_bucket_iam_member" "eval_artifact_writer" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${module.eval_service_account.email}"
}
```

## Cloud Run Job 変更

Job args は変えず、env だけで GCS 保存を有効化する。

```text
ENTRYPOINT /app/synthify-eval
ARGS --cases apps/eval/cases --format json
ENV EVAL_OUTPUT_GCS_URI=gs://.../eval/{env}/runs
```

これによりローカルでは GCS 保存なし、Cloud Run Job では自動保存になる。

## Report への追記

GCS 保存先を人間が stdout / Cloud Logging から追えるように、upload 成功時は stderr ではなく stdout の JSON 外にログを混ぜない。

代わりに report wrapper を導入する案もあるが、現行 report schema が `[]Result` なので今回は変えない。
CLI は upload 成功時に `log.Printf("eval artifact uploaded: %s", uri)` を出す。これは stderr に出るため JSON stdout を壊さない。

将来 dashboard が必要になったら、report schema を次のような wrapper に移行する。

```json
{
  "artifact_uri": "gs://...",
  "results": [...]
}
```

現段階では schema 破壊を避ける。

## テスト

Unit test:

- `gs://bucket/prefix` の parse
- `gs://bucket/prefix/` の trailing slash 正規化
- `http://...` や bucket なし URI を拒否
- run object path が `{yyyy}/{mm}/{dd}/{run_id}.json` になる
- `--out-gcs` が `EVAL_OUTPUT_GCS_URI` より優先される
- `--out-gcs` 未指定かつ env 未設定なら upload しない

GCS upload 自体は interface 化して fake writer でテストする。実 GCS integration test は通常 CI では走らせない。

Manual smoke:

```bash
go run ./apps/eval/cmd \
  --cases apps/eval/cases \
  --format json \
  --out-gcs gs://<dev-bucket>/eval/local/runs
```

Cloud Run smoke:

```bash
gcloud run jobs execute synthify-eval-stage \
  --region asia-northeast1 \
  --wait

gsutil ls gs://synthify-stage-491705-synthify-uploads-stage/eval/stage/runs/**
```

## 未対応

- `latest.json` の更新
- report retention / lifecycle rule
- BigQuery load / dashboard 集計
- golden artifact との比較
- Slack / GitHub 通知

これらは artifact が安定して保存されてから追加する。
