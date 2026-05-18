# Prompt / GCS URL Contract

このドキュメントは、prompt optimization loop 周辺で使う prompt の所在、variant 名、GCS artifact URI の最小契約をまとめる。

詳細契約:

- [prompt-variant-eval-contract.md](prompt-variant-eval-contract.md)
- [llm-eval-gcs-artifact-contract.md](llm-eval-gcs-artifact-contract.md)
- [llm-eval-runner-contract.md](llm-eval-runner-contract.md)

## 1. Prompt の所在

Production prompt の source of truth は repo file であり、GCS ではない。

```text
apps/worker/pkg/worker/prompts/templates/
  knowledge_tree.system.tmpl
  knowledge_tree.user.tmpl
```

Worker は `go:embed` された production prompt を使う。runtime で GCS prompt registry や管理 DB を読まない。

Variant prompt は eval 専用で、production worker image には混入させない。

```text
apps/eval/variants/{variant_name}/
  knowledge_tree.system.tmpl
  knowledge_tree.user.tmpl
```

`{variant_name}` は path segment として扱う。`/` を含めて階層を増やす名前は禁止する。

## 2. Prompt Source

Eval report は、どの prompt で実行したかを `prompt_source` で表す。

| 種別 | report payload | GCS path segment |
| :--- | :--- | :--- |
| production | `production` | `production` |
| variant | `variant:{variant_name}` | `variant-{variant_name}` |

GCS path segment では、`:` と `/` を `-` に正規化し、単一 path segment にする。

例:

| report payload | GCS path segment |
| :--- | :--- |
| `production` | `production` |
| `variant:concise-v1` | `variant-concise-v1` |

## 3. GCS Artifact URI

GCS に保存するのは prompt 本体ではなく、eval report artifact である。

Cloud Run Job / CLI の保存先 prefix:

```text
EVAL_OUTPUT_GCS_URI=gs://{bucket}/eval/{environment}/runs
```

Run ごとの object URI:

```text
gs://{bucket}/eval/{environment}/runs/{prompt_source}/{yyyy}/{mm}/{dd}/{run_id}.json
```

例:

```text
gs://synthify-stage-491705-synthify-uploads-stage/eval/stage/runs/production/2026/05/17/20260517T042000Z-a1b2c3.json
gs://synthify-stage-491705-synthify-uploads-stage/eval/stage/runs/variant-concise-v1/2026/05/18/20260518T000000Z-deadbe.json
```

Rules:

- `gs://` scheme のみ許可する。
- bucket は必須。
- prefix は slash 正規化し、末尾 slash の有無で保存先を変えない。
- `{prompt_source}` segment は必須。空なら `production` として扱う。
- object は run ごとに一意にする。
- 既存 object は上書きしない。
- `latest.json` のような mutable pointer は作らない。

## 4. CLI / Env

Eval CLI:

```bash
go run ./apps/eval/cmd \
  --cases apps/eval/cases \
  --format json \
  --out-gcs gs://bucket/eval/stage/runs
```

Variant 実行:

```bash
go run ./apps/eval/cmd \
  --cases apps/eval/cases \
  --variant concise-v1 \
  --format json \
  --out-gcs gs://bucket/eval/stage/runs
```

保存先の解決優先順位:

1. `--out-gcs`
2. `EVAL_OUTPUT_GCS_URI`
3. 未設定なら GCS 保存しない

`--out` と `--out-gcs` は併用できる。stdout は JSON report 専用に保ち、GCS URI の log は stderr / standard logger に出す。

## 5. Report Payload

GCS に保存する bytes は、stdout / `--out` に出す JSON report と同一にする。

Artifact wrapper は作らない。保存先 URI を JSON report の外側に追加しない。

Prompt / GCS URL に関わる必須 field:

```json
{
  "case_name": "knowledge_tree_api_spec",
  "tool": "knowledge_tree",
  "prompt_source": "production",
  "golden_checked": true,
  "golden_match": true
}
```

Variant の場合:

```json
{
  "case_name": "knowledge_tree_api_spec",
  "tool": "knowledge_tree",
  "prompt_source": "variant:concise-v1"
}
```

`golden_diff` を含む場合も payload の一部としてそのまま保存する。GCS artifact layer は diff の中身を解釈しない。

## 6. IAM

Eval runtime service account:

```text
synthify-eval-{environment}@...
```

必要権限:

| Resource | Role | 理由 |
| :--- | :--- | :--- |
| uploads bucket | `roles/storage.objectCreator` | run artifact を新規作成する |
| Secret Manager `synthify-gemini-api-key` | `roles/secretmanager.secretAccessor` | eval LLM call に必要 |

`roles/storage.objectAdmin` は付与しない。object overwrite / delete / latest pointer 更新をしないため不要。

## 7. スコープ外

- Production prompt を GCS prompt registry / 管理 DB に置くこと。
- Generated variant を GCS に保存して自動採用すること。
- BigQuery / dashboard 取り込み。
- Slack / GitHub 通知。
- Document upload / worker cache / checkpoint 用 GCS URL。
