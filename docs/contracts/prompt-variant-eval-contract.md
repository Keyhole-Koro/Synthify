# Prompt Variant Eval Contract

このドキュメントは、prompt optimization loop の「最初の実装単位」の契約を定義する。

1. `knowledge_tree` prompt 外出し（prompt renderer + `go:embed`）
2. eval runner の `--variant`

> **2026-05-20 注記**: 元は 3 項目目に「golden diff（`--golden` / `--update-golden`）」を含んでいたが、eval runner の道A 移行（全 tool = `Tool{Name,IOSchema,Run}` 単一概念、`apps/eval/runner`）で golden 機能は eval runner の責務から外された（[llm-eval-runner.md](../improvements/llm-eval-runner.md)）。本契約の §4 (旧 Golden 契約) は削除済み、§5/§6 から golden 関連の記述も除去している。

設計の背景は [llm-prompt-optimization-loop.md](../improvements/llm-prompt-optimization-loop.md)、
評価実行基盤の既存契約は [llm-eval-runner-contract.md](llm-eval-runner-contract.md) を参照する。
Analyst LLM / Prompt Writer LLM / BI Eval Review は本契約のスコープ外であり、基盤安定後に別契約として追加する。

この契約は実装の確定仕様である。実装が本契約と乖離した場合、実装か本契約のどちらかを直す。

## 0. 命名規約

tool 名・prompt・render の各レイヤーで同じ語を使い回さない（旧コードは `synthesis` を tool 実行・prompt 生成・テンプレート名に多重利用していて読めなかった）。以下を固定し、新規コードで混同しない。

| レイヤー | 語 | 具体 |
| :--- | :--- | :--- |
| ドメイン tool（LLM を実行する） | **knowledge tree generation** | `process.GenerateKnowledgeTree` / `ToolKnowledgeTree` / case の `tool: knowledge_tree`。tool の固有名であり改名しない |
| prompt を組み立てるもの | **Renderer** | `prompts.Renderer`。storage（embed / 将来の GCS）を隠す。`Render()` を持つ |
| render の入力 | **RenderInput** | `prompts.RenderInput`。`process.GenerateKnowledgeTreeArgs` と同じフィールドを写す |
| render の出力 | **Prompt** | `prompts.Prompt`（System / User） |
| tool 1 個分の prompt 一式を束ねるキー | **tool key** | `"knowledge_tree"`。`knowledge_tree.system.tmpl` / variant dir / case の `tool:` 値はこのキーで揃える |

「tool を実行する」(`GenerateKnowledgeTree`) と「その prompt 文字列を組む」(`Renderer.Render`) を同名にしない。これが本契約の命名上の不変条件である。

## 1. コンポーネントと責任

| コンポーネント | 所在 | 責任 |
| :--- | :--- | :--- |
| Prompt Renderer | `apps/worker/pkg/worker/prompts` | `go:embed` した production prompt template を render して返す（型は `prompts.Renderer`）。source of truth は repo file |
| Prompt templates | `apps/worker/pkg/worker/prompts/templates` | `knowledge_tree.system.tmpl` / `knowledge_tree.user.tmpl`。production prompt の唯一の正本 |
| Worker process tool | `apps/worker/pkg/worker/tools/builtin/process` | `GenerateKnowledgeTree` は hardcoded prompt をやめ、renderer が render した prompt を使う |
| Eval Runner | `apps/eval/runner` | `--variant` 指定時は variant prompt、未指定時は production prompt で `GenerateKnowledgeTree` を実行する。判定は output JSON の schema validation と JSON rule（道A、[llm-eval-runner.md](../improvements/llm-eval-runner.md)） |
| Eval CLI | `apps/eval/cmd` | `--variant` flag を解釈し runner に渡す |
| Variant store | `apps/eval/variants/{name}/` | 手書き variant template。production image に混入させない |

依存方向は `apps/eval` → `apps/worker/pkg/worker/prompts` の一方向に限定する（[dependency-architecture-ideal.md](../improvements/dependency-architecture-ideal.md) の理想構成と整合させ、worker は eval を import しない）。

## 2. Prompt Renderer 契約

### 2.1 Template layout

```text
apps/worker/pkg/worker/prompts/
  prompts.go
  templates/
    knowledge_tree.system.tmpl
    knowledge_tree.user.tmpl
```

- `prompts.go` は `//go:embed templates/*.tmpl` で template を埋め込む。実行時にファイルシステムや GCS を参照しない。
- template の source of truth は repo file で確定する。GCS prompt registry / 管理 DB 化は本契約のスコープ外。

### 2.2 Render 入力契約

renderer は次の入力から system / user prompt を render する。入力フィールドは現行 `process.GenerateKnowledgeTreeArgs` と一致させる。

| 入力 | 対応 | 用途 |
| :--- | :--- | :--- |
| `DocumentID` | `args.DocumentID` | user prompt の `document_id:` 行 |
| `Instruction` | `args.Instruction`（空なら `none`） | user prompt の `Instruction:` 行 |
| `Chunks` | `args.Chunks` | `[index] heading\ntext` 形式で連結し user prompt に埋める |
| `StylePrompt` (planned) | job snapshot / eval `input.style_prompt` | default style guide の後へ追加。空なら追加しない |

- 移行は出力同値を必須とする。template 化直後、および `StylePrompt` が空のときの system / user prompt
  は、現行 [knowledge_tree.go](../../apps/worker/pkg/worker/tools/builtin/process/knowledge_tree.go) の hardcoded
  文字列とバイト一致する（chunk 連結フォーマット `[%d] %s\n%s\n\n` と空 instruction の `none` 既定を含む）。
- prompt の意味的変更はこの移行 PR では行わない。挙動変更は variant または別 PR で行う。

### 2.3 Worker / Eval 双方の利用契約

- `process.GenerateKnowledgeTree` は renderer から render した prompt を `llm.StructuredRequest` に渡す。`Schema` と JSON parse / fallback（array 形 unmarshal、`no items` エラー）の挙動は現行と変えない。
- eval runner は variant 未指定時、worker と同一の renderer を呼ぶ。eval 専用に prompt を複製しない。

## 3. Variant 契約

### 3.1 Layout

```text
apps/eval/variants/
  {variant_name}/
    knowledge_tree.system.tmpl
    knowledge_tree.user.tmpl
```

- variant template は `apps/eval` 配下にのみ置き、production worker image（`apps/worker`）に含めない。
- variant template の render 入力契約は production template（2.2）と同一。入力契約を変える variant は許容しない。

### 3.2 CLI

| flag | 必須 | 内容 |
| :--- | :--- | :--- |
| `--variant` | 任意 | `apps/eval/variants/{name}` を prompt source にする。未指定時は production renderer を使う |

```bash
go run ./apps/eval/cmd --cases apps/eval/cases --variant concise-structure-v1 --format json
go run ./apps/eval/cmd --cases apps/eval/cases   # variant 未指定 = production prompt
```

- 指定 variant directory が存在しない、または必須 template を欠く場合は exit `2` で明示エラーにする（暗黙の production fallback はしない）。
- Cloud Run Job の通常 eval は `--variant` を付けない。定期 eval は常に production prompt で走る。

## 4. (削除済み) 旧 Golden 契約

> **2026-05-20 削除**: eval runner の道A 移行で golden 機能は runner 責務から外された
> （[llm-eval-runner.md](../improvements/llm-eval-runner.md) 「golden 判定、golden 更新、golden diff は現行 runner の責務から外す」）。
>
> 旧 §4 は `apps/eval/golden/` ディレクトリ + `--golden` / `--update-golden` フラグ +
> tree 固有 strict diff（item_count / title_set / parent_structure / source_chunk_ids / max_depth）
> を規定していたが、対応する実装（`apps/eval/runner/golden.go`、`apps/eval/golden/*.json`、
> CLI フラグ）はすべて削除済み。
>
> 道A の判定は **output JSON の JSON Schema 適合 + JSON rule（`expect.json`: count_gte /
> tree_depth_lte / contains_all 等）** に統一されている。旧 §4 の strict フィールドは
> JSON rule で概ね表現できる（item count → `count_gte`、max depth → `tree_depth_lte`、
> title set → `contains_all`）。parent structure と source_chunk_ids の strict 検証は
> 現状の JSON rule op では非対応 — 必要になったら op を足す（YAGNI）。
>
> 旧設計の意図（同じ case set で variant と production を比較するための固定期待値）は、
> 道A では「case YAML の `expect.json` 述語が固定期待値である」ことで等価に達成される。

## 5. Failure / Exit Code 契約

既存 eval の exit code（[llm-eval-runner-contract.md](llm-eval-runner-contract.md) §2）を以下で拡張する。

| 条件 | exit code |
| :--- | :--- |
| flag 不正、variant directory 不在、case 読み込み不能 | `2` |
| すべての case が schema 適合かつ JSON rule pass | `0` |
| 1 件以上の case が schema 不適合 / rule fail / tool error | `1` |

## 6. Report Payload 契約

case ごとの Result（[runner.go](../../apps/eval/runner/runner.go) の `Result`）は道A 移行で
golden 関連フィールド（`golden_checked` / `golden_match` / `golden_diff`）と tree 固有
フィールド（`Items` / `ItemCount` / `MaxDepth` / `MissingTitle`）を削除し、tool 非依存の
output JSON に統一済み。

| field | 型 | 意味 |
| :--- | :--- | :--- |
| `output` | json.RawMessage | tool が返した output JSON そのまま（tool 非依存） |
| `schema_valid` | bool | output が tool の宣言 IOSchema に適合したか（道A の共通最低判定） |
| `prompt_source` | string | `production` または `variant:{name}` |
| `passed` | bool | schema_valid かつ `expect.json` の全 rule が pass |
| `error` | string | tool-level error（LLM error / no items / transform 非OK）。空なら成功 |
| `failed_input` | object | 失敗時に添付される入力スナップショット（prepare 層が組む） |

- JSON report の HTML escape 方針（`<` にしない）は既存契約を踏襲する。
- `prompt_source` を必ず出すことで、baseline / variant のどちらの結果かを report 単体で判別できる。

## 7. Test 契約

- prompt renderer: render 出力が現行 hardcoded prompt とバイト一致する regression test を置く（移行同値の保証。`prompts_test.go` の `TestDefaultRendererMatchesLegacyPrompt`。これは「prompts package の固定期待値テスト」であって eval runner の golden 機能ではない）。
- variant: 不在 variant 指定で exit `2`、有効 variant 指定で `prompt_source=variant:{name}` になることを test する。
- 道A 判定: schema 適合 / 不適合、JSON rule pass / fail、tool error の各経路を test する。
- 既存 eval test（rule 判定、exit code）の挙動を回帰させないこと（API は道A で変わったが pass/fail/exit の振る舞いは維持）。

## 8. スコープ外

本契約は単一 tool（`knowledge_tree`）に閉じて正しい。マルチツール化は別レイヤー（eval 実装の「道A」: 全 tool を `Tool{Name,IOSchema,Run}` 単一概念に統一、`apps/eval/runner`）が引き受ける。別契約 md は廃止し設計根拠はコード doc コメントに保全。`--variant` / `prompt_source` は道A でも knowledge_tree builtin tool として動く（renderer はクロージャ捕捉）。`Result` payload と判定方法は道A で再定義済み（§6 / §4 注記）。

- Analyst LLM / Prompt Writer LLM の出力契約。
- BI Eval Review の `prompt_variant_reviews` model と approve / apply API。
- `knowledge_tree` 以外の tool（`summary` / `briefing` / `critique` / `merging`）の prompt 外出し。
- eval runner のマルチツール対応（→ eval 実装「道A」が所有。`apps/eval/runner`、別契約 md は廃止）。
- GCS prompt registry / 管理 DB への移行。
- generated variant（`apps/eval/variants/generated/`）の扱い。手書き variant のみ本契約の対象とする。
