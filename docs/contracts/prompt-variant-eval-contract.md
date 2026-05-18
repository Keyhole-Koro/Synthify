# Prompt Variant Eval Contract

このドキュメントは、prompt optimization loop の「最初の実装単位」3 つの契約を定義する。

1. `synthesis` prompt 外出し（prompt provider + `go:embed`）
2. eval runner の `--variant`
3. golden diff（`--golden` / `--update-golden`）

設計の背景は [llm-prompt-optimization-loop.md](../improvements/llm-prompt-optimization-loop.md)、
評価実行基盤の既存契約は [llm-eval-runner-contract.md](llm-eval-runner-contract.md) を参照する。
Analyst LLM / Prompt Writer LLM / BI Eval Review は本契約のスコープ外であり、基盤安定後に別契約として追加する。

この契約は実装の確定仕様である。実装が本契約と乖離した場合、実装か本契約のどちらかを直す。

## 1. コンポーネントと責任

| コンポーネント | 所在 | 責任 |
| :--- | :--- | :--- |
| Prompt Provider | `apps/worker/pkg/worker/prompts` | `go:embed` した production prompt template を render して返す。source of truth は repo file |
| Prompt templates | `apps/worker/pkg/worker/prompts/templates` | `synthesis.system.tmpl` / `synthesis.user.tmpl`。production prompt の唯一の正本 |
| Worker process tool | `apps/worker/pkg/worker/tools/process` | `Synthesize` は hardcoded prompt をやめ、provider が render した prompt を使う |
| Eval Runner | `apps/eval/runner` | `--variant` 指定時は variant prompt、未指定時は production prompt で `synthesize` を実行する。golden と突き合わせる |
| Eval CLI | `apps/eval/cmd` | `--variant` / `--golden` / `--update-golden` flag を解釈し runner に渡す |
| Variant store | `apps/eval/variants/{name}/` | 手書き variant template。production image に混入させない |
| Golden store | `apps/eval/golden/{case_name}.json` | case ごとの期待 output。strict 判定対象フィールドのみを保持する |

依存方向は `apps/eval` → `apps/worker/pkg/worker/prompts` の一方向に限定する（[dependency-architecture-ideal.md](../improvements/dependency-architecture-ideal.md) の理想構成と整合させ、worker は eval を import しない）。

## 2. Prompt Provider 契約

### 2.1 Template layout

```text
apps/worker/pkg/worker/prompts/
  prompts.go
  templates/
    synthesis.system.tmpl
    synthesis.user.tmpl
```

- `prompts.go` は `//go:embed templates/*.tmpl` で template を埋め込む。実行時にファイルシステムや GCS を参照しない。
- template の source of truth は repo file で確定する。GCS prompt registry / 管理 DB 化は本契約のスコープ外。

### 2.2 Render 入力契約

provider は次の入力から system / user prompt を render する。入力フィールドは現行 `process.SynthesisArgs` と一致させる。

| 入力 | 対応 | 用途 |
| :--- | :--- | :--- |
| `DocumentID` | `args.DocumentID` | user prompt の `document_id:` 行 |
| `Instruction` | `args.Instruction`（空なら `none`） | user prompt の `Instruction:` 行 |
| `Chunks` | `args.Chunks` | `[index] heading\ntext` 形式で連結し user prompt に埋める |

- 移行は出力同値を必須とする。template 化直後の system / user prompt は、現行 [synthesis.go](../../apps/worker/pkg/worker/tools/process/synthesis.go) の hardcoded 文字列とバイト一致する（chunk 連結フォーマット `[%d] %s\n%s\n\n` と空 instruction の `none` 既定を含む）。
- prompt の意味的変更はこの移行 PR では行わない。挙動変更は variant または別 PR で行う。

### 2.3 Worker / Eval 双方の利用契約

- `process.Synthesize` は provider から render した prompt を `llm.StructuredRequest` に渡す。`Schema` と JSON parse / fallback（array 形 unmarshal、`no items` エラー）の挙動は現行と変えない。
- eval runner は variant 未指定時、worker と同一の provider を呼ぶ。eval 専用に prompt を複製しない。

## 3. Variant 契約

### 3.1 Layout

```text
apps/eval/variants/
  {variant_name}/
    synthesis.system.tmpl
    synthesis.user.tmpl
```

- variant template は `apps/eval` 配下にのみ置き、production worker image（`apps/worker`）に含めない。
- variant template の render 入力契約は production template（2.2）と同一。入力契約を変える variant は許容しない。

### 3.2 CLI

| flag | 必須 | 内容 |
| :--- | :--- | :--- |
| `--variant` | 任意 | `apps/eval/variants/{name}` を prompt source にする。未指定時は production provider を使う |

```bash
go run ./apps/eval/cmd --cases apps/eval/cases --variant concise-structure-v1 --format json
go run ./apps/eval/cmd --cases apps/eval/cases   # variant 未指定 = production prompt
```

- 指定 variant directory が存在しない、または必須 template を欠く場合は exit `2` で明示エラーにする（暗黙の production fallback はしない）。
- Cloud Run Job の通常 eval は `--variant` を付けない。定期 eval は常に production prompt で走る。

## 4. Golden 契約

### 4.1 Layout と判定対象

```text
apps/eval/golden/
  {case_name}.json
```

ファイル名は case の `name` と一致させる（例: `synthesis_api_spec.json`）。

strict 判定に含めるフィールド:

- item count
- title set
- parent structure（`local_id`, `parent_local_id`）
- `source_chunk_ids`
- max depth

strict 判定に含めないフィールド:

- `content` HTML 全文
- description の表現差分
- item ordering の軽微な差分

`source_chunk_ids` を strict に含めるのは、入力 testdata（chunk 分割・順序）が固定である前提に依存する。testdata を変更した場合は golden 再生成が必要であり、これを破壊的変更として扱う。

### 4.2 CLI

| flag | 必須 | 内容 |
| :--- | :--- | :--- |
| `--golden` | 任意 | golden directory。指定時、各 case を 4.1 の strict フィールドで突き合わせる |
| `--update-golden` | 任意 | 現在の出力で golden を書き出す。`--golden` と排他にせず、本 flag 指定時は判定ではなく書き出しを行う |

```bash
go run ./apps/eval/cmd --cases apps/eval/cases --golden apps/eval/golden
go run ./apps/eval/cmd --cases apps/eval/cases --update-golden apps/eval/golden
```

- `--golden` 未指定時の挙動は既存契約（[llm-eval-runner-contract.md](llm-eval-runner-contract.md) の rule 判定）と同一で、golden 判定を行わない。
- golden 書き出しは `--update-golden` 明示時のみ。`--golden` 判定時に golden を自動更新しない。
- 初回 golden は無条件採用しない。`--update-golden` で雛形を生成し、人間が strict フィールドを確認・修正して commit する。以降の更新も diff を人間が確認したうえで commit する前提とする（CI 上で `--update-golden` を自動実行しない）。

## 5. Failure / Exit Code 契約

既存 eval の exit code（[llm-eval-runner-contract.md](llm-eval-runner-contract.md) §2）を以下で拡張する。

| 条件 | exit code |
| :--- | :--- |
| flag 不正、variant directory 不在、case 読み込み不能 | `2` |
| すべての case が rule pass かつ（`--golden` 時）golden match | `0` |
| 1 件以上の case が rule fail、または golden mismatch | `1` |
| `--update-golden` で全 case 書き出し成功 | `0` |

- golden mismatch は exit `1` とする。warn mode の要否は未決定（[llm-prompt-optimization-loop.md](../improvements/llm-prompt-optimization-loop.md) の Open Questions）。本契約では mismatch = fail を既定とする。
- variant・production 双方が同一 case で golden mismatch する場合も exit `1` だが、report 上はその case を golden 自体の要再レビュー候補として識別できる情報を残す（§6）。

## 6. Report Payload 契約

§4.1 の判定を行った場合、case ごとの Result に golden 判定結果を追加する。既存 `Result`（[runner.go](../../apps/eval/runner/runner.go) の `Result`）の互換を壊さず additive に拡張する。

| field | 型 | 意味 |
| :--- | :--- | :--- |
| `golden_checked` | bool | `--golden` 指定で判定対象だったか |
| `golden_match` | bool | strict フィールドが golden と一致したか |
| `golden_diff` | object | mismatch したフィールドと expected / actual の要約。`content` 全文は含めない |
| `prompt_source` | string | `production` または `variant:{name}` |

- JSON report の HTML escape 方針（`<` にしない）は既存契約を踏襲する。
- `prompt_source` を必ず出すことで、baseline / variant のどちらの結果かを report 単体で判別できる。

## 7. Test 契約

- prompt provider: render 出力が現行 hardcoded prompt とバイト一致する golden test を置く（移行同値の保証）。
- variant: 不在 variant 指定で exit `2`、有効 variant 指定で `prompt_source=variant:{name}` になることを test する。
- golden: match / mismatch / `--update-golden` 書き出しの 3 経路を test する。mismatch 時に exit `1` かつ `golden_diff` が `content` 全文を含まないことを assert する。
- 既存 eval test（rule 判定、exit code）の挙動を回帰させないこと。

## 8. スコープ外

- Analyst LLM / Prompt Writer LLM の出力契約。
- BI Eval Review の `prompt_variant_reviews` model と approve / apply API。
- `synthesis` 以外の tool（`summary` / `briefing` / `critique` / `merging`）の prompt 外出し。
- GCS prompt registry / 管理 DB への移行。
- generated variant（`apps/eval/variants/generated/`）の扱い。手書き variant のみ本契約の対象とする。
