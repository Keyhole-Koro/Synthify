# LLM Eval GCS Artifact Contract

このドキュメントは、LLM Eval Runner の report を GCS に保存するためのコンポーネント契約を定義する。

## 1. 目的と責任境界

Eval GCS Artifact は、Cloud Run Job またはローカル CLI で実行された eval report を、後から参照可能な immutable artifact として GCS に保存する。

この契約で扱うもの:

- Eval CLI が生成した JSON report bytes
- GCS object path の決定
- GCS upload の成功/失敗扱い
- Cloud Run Job への保存先注入
- eval runtime service account の最小 IAM

この契約で扱わないもの:

- `latest.json` のような上書き pointer（不要。§2 の object path は `{run_id}` がタイムスタンプ先頭のため、`runs/{prompt_source}/` 配下を lexical sort すれば最新が末尾に決まる。mutable pointer を足すと §6 の最小 IAM = `objectCreator` のみ・immutable artifact 原則を崩し、並行 run の上書き競合も生むため、意図的に持たない）
- golden diff の**判定ロジック**（[prompt-variant-eval-contract.md](prompt-variant-eval-contract.md) の責任。GCS artifact は report payload の一部として `golden_diff` を**そのまま運ぶだけ**で、判定はしない）
- report payload の**スキーマ定義**（同じく prompt-variant-eval-contract / llm-eval-runner-contract の責任。本契約は bytes を運ぶことだけに責任を持つ）
- BigQuery / dashboard 取り込み
- Slack / GitHub 通知
- document upload / worker cache / checkpoint の保存

## 2. 保存先 URI 契約

Cloud Run Job は次の env を受け取る。

```text
EVAL_OUTPUT_GCS_URI=gs://{bucket}/eval/{environment}/runs
```

CLI はこの prefix 配下に run ごとの object を作成する。object path には
`prompt_source` segment を含め、baseline と variant の run を URI だけで分離する。

```text
gs://{bucket}/eval/{environment}/runs/{prompt_source}/{yyyy}/{mm}/{dd}/{run_id}.json
```

`{prompt_source}` は report payload の `prompt_source`（[prompt-variant-eval-contract.md](prompt-variant-eval-contract.md) §6）と一致する 1 segment:

- production prompt の run → `production`
- variant の run → `variant-{name}`（payload では `variant:{name}`。`:` と `/` は `-` に正規化し、余分な path level を作らない）

例:

```text
gs://synthify-stage-491705-synthify-uploads-stage/eval/stage/runs/production/2026/05/17/20260517T042000Z-a1b2c3.json
gs://synthify-stage-491705-synthify-uploads-stage/eval/stage/runs/variant-concise-v1/2026/05/18/20260518T000000Z-deadbe.json
```

Rules:

- `EVAL_OUTPUT_GCS_URI` は `gs://` scheme のみ許可する。
- bucket は必須。
- prefix は slash で正規化し、末尾 slash の有無で保存先が変わってはならない。
- `prompt_source` segment は必須。空・未指定の run は `production` として保存する。
- `prompt_source` の `:` `/` は単一 segment に正規化し、path 階層を増やさない。
- object は run ごとに一意でなければならない。
- 既存 object の上書きは行わない。
- baseline / variant の比較は同じ `runs/` 配下の `production/` と `variant-{name}/` を突き合わせて行う。[prompt_variant_reviews](../improvements/llm-prompt-optimization-loop.md) の `run_artifact_uri` / `baseline_artifact_uri` はこの URI 規約で解決する。

## 3. CLI 契約

Eval CLI は GCS 保存用に次の flag を持つ。

```bash
--out-gcs gs://bucket/prefix
```

保存先の解決優先順位:

1. `--out-gcs`
2. `EVAL_OUTPUT_GCS_URI`
3. 未設定なら GCS 保存しない

`--out` と `--out-gcs` は併用できる。stdout 出力は常に維持する。

```bash
synthify-eval \
  --cases apps/eval/cases \
  --format json \
  --out local-result.json \
  --out-gcs gs://bucket/eval/stage/runs
```

GCS 保存に成功した場合、CLI は JSON stdout を壊さない形で保存先 URI を log に出す。
stdout は report JSON 専用とし、GCS URI の人間向け log は stderr / standard logger に出す。

## 4. Report Payload 契約

GCS に保存する payload は、stdout / `--out` に出す JSON report と同一 bytes とする。

**payload schema の source of truth は本契約ではない。** schema は
[prompt-variant-eval-contract.md](prompt-variant-eval-contract.md) §6（`prompt_source` /
`golden_checked` / `golden_match` / `golden_diff`）と
[llm-eval-runner-contract.md](llm-eval-runner-contract.md) §4 が定義する。本契約は
それを bytes として運ぶことだけに責任を持ち、schema をここで二重定義しない。

現行 report 例（schema の正本は上記契約。ここは形のイメージ）:

```json
[
  {
    "case_name": "knowledge_tree_api_spec",
    "tool": "knowledge_tree",
    "passed": true,
    "schema_valid": true,
    "item_count": 4,
    "max_depth": 2,
    "missing_titles": null,
    "prompt_source": "production",
    "golden_checked": true,
    "golden_match": true,
    "duration_ms": 11838,
    "model": "gemini-3-flash-preview",
    "input_tokens": 544,
    "output_tokens": 990,
    "items": []
  }
]
```

Rules:

- GCS artifact は JSON report そのものだけを保存する。
- artifact URI を report wrapper として埋め込まない。
- `prompt_source` は必ず含まれる（baseline / variant 判別の根幹であり、§2 の path segment と一致する）。
- `golden_diff` を含む場合も payload の一部として運ぶだけで、本契約は中身を解釈しない（full `content` HTML を含まないことは判定側契約 §6 が保証）。
- HTML content は `\u003c` などに escape しない。
- fail した case では既存 report 契約通り `failed_input` を含めてよい。

## 5. Upload 実装契約

GCS upload は `apps/eval/artifact` が担当する。

想定責任:

- `gs://` URI parse
- prefix 正規化
- run object name 生成
- `cloud.google.com/go/storage` writer による object create
- upload 先 URI の返却

既存実装の流用方針:

| 既存実装 | 判断 |
| :--- | :--- |
| worker の `storage.FileSystem` | 使わない。GCS FUSE/local mount 前提で、Cloud Run Job の artifact 保存には重い |
| `WriteCache` / `WriteCheckpoint` | 使わない。worker 内部状態用で、eval artifact の責任領域と違う |
| `BuildDocumentUploadURL` / `BuildDocumentSourceURL` | 使わない。document upload/source URL 専用 |
| `NewGCSSignedDocumentUploadURLIssuer` | 使わない。eval Job 自身が service account で直接書くため署名 URL は不要 |
| `app.Bootstrap` | 使わない。eval CLI は DB / notifier / store を必要としない |

Required behavior:

- object content type は `application/json`。
- upload は context cancellation を尊重する。
- GCS client / writer close error は失敗として扱う。
- upload error は CLI まで返し、eval run の exit code を `1` にする。

## 6. IAM 契約

eval runtime service account:

```text
synthify-eval-{environment}@...
```

必要権限:

| Resource | Role | 理由 |
| :--- | :--- | :--- |
| uploads bucket | `roles/storage.objectCreator` | run artifact を新規作成する |
| Secret Manager `synthify-gemini-api-key` | `roles/secretmanager.secretAccessor` | eval LLM call に必要 |

`roles/storage.objectAdmin` は付与しない。初期実装では object の上書き・削除・latest 更新を行わないため不要。

GCS IAM は prefix 単位では切れないため、bucket level で `objectCreator` を付与し、アプリ側で `eval/{environment}/runs` prefix に固定する。

## 7. Terraform / Cloud Run Job 契約

`terraform/services/eval` は次の input を受ける。

```hcl
eval_output_gcs_uri = "gs://${module.platform.uploads_bucket_name}/eval/${var.environment}/runs"
```

Cloud Run Job env:

| Env | 値 |
| :--- | :--- |
| `GEMINI_MODEL` | `var.gemini_model` |
| `GEMINI_API_KEY` | Secret Manager |
| `EVAL_OUTPUT_GCS_URI` | `gs://{uploads_bucket}/eval/{environment}/runs` |

Cloud Run Job args は GCS の有無に関わらず変えない。

```text
--cases apps/eval/cases --format json
```

これにより、ローカル実行は GCS 保存なし、Cloud Run Job は env により GCS 保存ありになる。

## 8. Failure Contract

| Failure | 挙動 |
| :--- | :--- |
| eval case が fail | report を stdout に出す。GCS 保存先があれば保存を試みる。最終 exit code は `1` |
| GCS URI 不正 | report は stdout に出す。GCS 保存エラーとして exit code `1` |
| GCS upload 失敗 | report は stdout に出す。エラーを stderr/log に出し exit code `1` |
| GCS env 未設定 | GCS 保存をスキップし、eval result の合否だけで exit code を決める |
| context timeout | 実行中の eval / upload を中断し exit code `1` |

重要: GCS 保存に失敗しても stdout report は可能な限り出す。これにより Cloud Logging から最低限の結果を回収できる。

## 9. Test Contract

Unit tests:

- valid `gs://bucket/prefix` を parse できる
- trailing slash を正規化する
- bucket なし、`http://`、空 URI を拒否する
- generated object name が `{prompt_source}/{yyyy}/{mm}/{dd}/{run_id}.json` になる
- `prompt_source` 未指定で `production/` segment になる
- `variant:concise-v1` が `variant-concise-v1/` の単一 segment に正規化される（path 階層を増やさない）
- `--out-gcs` が `EVAL_OUTPUT_GCS_URI` より優先される
- GCS 未設定なら upload を呼ばない
- fake uploader の error が CLI exit failure に反映される

Integration / smoke:

```bash
go run ./apps/eval/cmd \
  --cases apps/eval/cases \
  --format json \
  --out-gcs gs://<dev-bucket>/eval/local/runs
```

Cloud Run:

```bash
gcloud run jobs execute synthify-eval-stage \
  --region asia-northeast1 \
  --wait

gsutil ls gs://synthify-stage-491705-synthify-uploads-stage/eval/stage/runs/**
```
