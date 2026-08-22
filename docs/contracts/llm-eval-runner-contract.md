# LLM Eval Runner Contract

このドキュメントは、LLM Eval Runner を構成する CLI、ケース定義、Cloud Run Job、Cloud Scheduler、Terraform/CD の契約を定義する。
Eval Runner は本番 worker のプロンプト・モデル変更による出力変化を検知するための評価実行基盤であり、ユーザー向け API ではない。

## 1. コンポーネントと責任

| コンポーネント | 所在 | 責任 |
| :--- | :--- | :--- |
| Eval CLI | `apps/eval/cmd` | case を読み込み、LLM tool eval を実行し、report を stdout / `--out` に出力する |
| Eval Runner | `apps/eval/runner` | YAML case / fixture を解決し、`knowledge_tree` を実行し、schema/rule/golden 以外の現行スコアを計算する |
| Report Writer | `apps/eval/report` | `table` / `json` report を生成する。JSON は HTML を `\u003c` 等に escape しない |
| Worker process tool | `apps/worker/pkg/worker/tools/builtin/process` | 本番と同じ `GenerateKnowledgeTree` API を提供する。eval は ADK を通さずこの API を直接呼ぶ |
| Eval image | `apps/eval/Dockerfile` | CLI binary、`apps/eval/cases`、`apps/eval/testdata` を含む Cloud Run Job 用 image を作る |
| Cloud Run Job | `terraform/services/eval` | eval image を 1 task / retry なしで実行する。結果は Cloud Logging に stdout として残す。GCS artifact 保存は別契約を参照 |
| Cloud Scheduler | `terraform/services/eval` | cron で Cloud Run Job の `:run` endpoint を呼ぶ |
| GitHub Actions CD | `.github/workflows/deploy-backend.yml` | eval image を build/push し、Terraform に `eval_image` を渡す |

## 2. CLI 契約

Eval CLI は次のどちらか一方を必須とする。

```bash
go run ./apps/eval/cmd --case apps/eval/cases/knowledge_tree_api_spec.yaml --format json
go run ./apps/eval/cmd --cases apps/eval/cases --format json --out apps/eval/results/latest.json
```

| flag | 必須 | 内容 |
| :--- | :--- | :--- |
| `--case` | `--cases` と排他 | 単一 YAML case file |
| `--cases` | `--case` と排他 | `*.yaml` / `*.yml` を列挙する directory |
| `--format` | 任意 | `json` または `table`。デフォルトは `table` |
| `--out` | 任意 | stdout と同じ report を保存する file path。親 directory は自動作成する |
| `--timeout` | 任意 | eval 全体の timeout。デフォルトは `60s` |

終了コード:

| 条件 | exit code |
| :--- | :--- |
| flag 不正、設定不足、case 読み込み不能 | `2` または `1` |
| すべての case が `passed=true` | `0` |
| 1 件以上の case が `passed=false` | `1` |

ローカル実行では repository root の `.env` を読み込む。既に同名の環境変数がある場合、`.env` は上書きしない。

## 3. Case / Fixture 契約

現行 MVP は `tool: knowledge_tree` のみ対応する。他 tool 名は明示エラーにする。

```yaml
name: knowledge_tree_api_spec
tool: knowledge_tree
input:
  document_id: doc_api_spec
  instruction: "技術仕様書として整理して"
  chunks: ../testdata/api_spec_chunks.json
expect:
  schema_valid: true
  json:
    - path: $.items
      op: count_gte
      value: 3
    - path: $.items
      op: tree_depth_lte
      value: 4
    - path: $.items[*].title
      op: contains_all
      value:
        - "認証"
        - "エラーハンドリング"
```

`expect` は道A（[llm-eval-runner.md](../improvements/llm-eval-runner.md)）で `schema_valid` と `json` rule に統一済み。旧 `min_items` / `max_depth` / `must_contain_titles` は廃止し、それぞれ `count_gte` / `tree_depth_lte` / `contains_all`（`$.items[*].title` に対する部分一致）で表現する。JSON path は軽量 subset（`$.x`, `$.x[*].y`, `$.x[i]`）のみを扱う。

`input.chunks` は case file の directory からの相対 path、または絶対 path とする。fixture は `[]domain.Chunk` JSON で固定する。PDF や raw document からの抽出は eval 対象に含めない。

## 4. Report 契約

JSON report は `[]Result` を stdout に出す。`--out` 指定時は同じ bytes を file に保存する。

主な field:

道A 移行で Result は tool 非依存の output JSON に統一済み。tree 固有 field（`item_count` / `max_depth` / `missing_titles` / `items`）は削除された（[prompt-variant-eval-contract.md](prompt-variant-eval-contract.md) §6）。

| field | 内容 |
| :--- | :--- |
| `case_name` / `tool` | 実行 case の識別子 |
| `passed` | `schema_valid` かつ `expect.json` の全 rule pass、かつ tool error なしの総合合否 |
| `schema_valid` | tool の宣言 IOSchema に output JSON が適合したか（道A の共通最低判定） |
| `output` | tool が返した output JSON そのまま（tool 非依存）。JSON report のみ実質レビュー対象 |
| `prompt_source` | `production` または `variant:{name}` |
| `duration_ms` | LLM call を含む case 実行時間 |
| `model` / `input_tokens` / `output_tokens` | LLM provider の usage |
| `error` | tool-level error（LLM call / parse error / no items）。空なら成功 |
| `failed_input` | fail 時のみ。document_id、instruction、chunks path、chunks 本体 |

`output` 内の HTML は可読性のため `<`, `>`, `&` を Unicode escape しない。

## 5. Cloud Run Job / Scheduler 契約

Cloud Run Job は常駐 HTTP service ではなく、実行して終了する batch job である。

| 項目 | 値 |
| :--- | :--- |
| Job name | `synthify-eval-${environment}` |
| Entrypoint | `/app/synthify-eval` |
| Args | `--cases apps/eval/cases --format json` |
| Task count / parallelism | `1 / 1` |
| Retry | `0` |
| Model | Terraform `gemini_model` → env `GEMINI_MODEL` |
| Output | Cloud Logging stdout |

Scheduler は `eval_schedule` / `eval_time_zone` に従い、Cloud Run Job の v2 `:run` endpoint を OAuth token 付きで呼ぶ。手動実行は次の形を使う。

```bash
gcloud run jobs execute synthify-eval-stage \
  --region asia-northeast1 \
  --wait
```

## 6. Terraform / IAM 契約

Platform module は次の service account を作る。

| Service Account | 用途 |
| :--- | :--- |
| `synthify-eval-${environment}` | Cloud Run Job runtime。Vertex AI を呼び出す |
| `synthify-eval-scheduler-${environment}` | Cloud Scheduler OAuth principal。Cloud Run Job を実行する |

IAM:

- eval runtime SA は project に `roles/aiplatform.user` を持つ。
- scheduler SA は eval job に `roles/run.invoker` を持つ。
- `deployer_principal` が設定されている場合、CI/WIF principal は eval runtime SA と scheduler SA に `roles/iam.serviceAccountUser` を持つ。

Terraform environment variables:

| 変数 | デフォルト | 内容 |
| :--- | :--- | :--- |
| `eval_image` | `""` | CD が SHA tag image を渡す |
| `eval_schedule` | `0 4 * * *` | 毎日 04:00 実行 |
| `eval_time_zone` | `Asia/Tokyo` | Scheduler timezone |

## 7. 現行スコープ外

- Agent eval (ADK orchestrator 経由)
- prompt variant 比較
- LLM-as-Judge
- golden diff / golden update
- Slack / GitHub issue などへの通知

これらを追加する場合も、CLI report schema と Cloud Run Job の exit code 契約を壊さないこと。

GCS への report 永続保存は [llm-eval-gcs-artifact-contract.md](llm-eval-gcs-artifact-contract.md) を参照する。

## 8. Model / style selection extension (planned)

モデル・スタイル選択を実装するときは
[model-and-style-selection-contract.md](model-and-style-selection-contract.md) §6 を additive extension の
source of truth とする。

- case の `input.style_prompt` は style guide 用。既存 `input.instruction` の chunk-specific instruction と
  混用しない。
- report は provider、requested/effective model selection、selection scope、style prompt hash を追加する。
  raw style prompt は artifact に保存しない。
- 現行 runner は ADK orchestrator を通らないため、process-tool output は評価できるが tool selection
  accuracy は評価できない。agent eval は別 harness / contract とする。
- field omission を許す additive change とし、既存 case、report reader、exit code semantics を壊さない。
