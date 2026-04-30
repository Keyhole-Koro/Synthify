# process/ ツール群の LLM 実装

## 追加で明記すべきこと（指示）

LLM に対して、以下を明示的に指示すること。

- ツリーアイテムの用途: 何を表すノードか、どの粒度で分割するか、親子関係の基準
- 期待する成果物: 最終的に生成されるツリーの品質条件、出力の必須フィールド、利用先(例: UI, 検索, 要約)

## 実装済み（完了）

---

## 前提：`base.Context` に `LLMClient` を追加

`process/` の各ツールが LLM を使えるよう、`base.Context` に `LLMClient` インターフェイスを追加した。

```go
// worker/pkg/worker/tools/base/base.go
type LLMClient interface {
    GenerateStructuredSimple(ctx context.Context, systemPrompt, userPrompt string, schema any) (json.RawMessage, error)
    GenerateTextSimple(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

type Context struct {
    Repo     Repository
    Embedder Embedder
    LLM      LLMClient   // 追加
    Memories []PromptMemory
}
```

既存の `llm.Client` は `StructuredRequest` 構造体を引数に取る設計（ファイル添付などに対応）だが、`process/` ツールからはシンプルな呼び出しで十分なため、`GeminiClient` に薄いラッパーを追加して `base.LLMClient` を満たした。

```go
// worker/pkg/worker/llm/gemini.go
func (c *GeminiClient) GenerateStructuredSimple(ctx, systemPrompt, userPrompt string, schema any) (json.RawMessage, error) {
    return c.GenerateStructured(ctx, StructuredRequest{SystemPrompt: systemPrompt, UserPrompt: userPrompt, Schema: schema})
}
func (c *GeminiClient) GenerateTextSimple(ctx, systemPrompt, userPrompt string) (string, error) {
    return c.GenerateText(ctx, TextRequest{SystemPrompt: systemPrompt, UserPrompt: userPrompt})
}
```

---

## 1. `generate_brief`

**場所**: `worker/pkg/worker/tools/process/briefing.go`

### 動作

`GenerateStructuredSimple` で `domain.DocumentBrief` スキーマを指定して構造化出力させる。

```
SystemPrompt:
  "You are a document analyst. Given a list of section headings, infer:
   - topic: the main subject (short phrase)
   - claim_summary: the core argument in one sentence
   - entities: key named concepts (up to 10)
   - level01_hints: suggested top-level labels for a knowledge tree (up to 5)
   - outline: the original headings unchanged"

UserPrompt:
  "Section headings:\n1. ...\n2. ..."
```

結果は `memory.Brief.Set()` に書き込まれ、次ターンから Working Memory に自動注入される。

**失敗時**: 見出しの先頭を Topic にした最小限の `DocumentBrief` をフォールバックとして返す。LLM エラーで処理が止まらない。

---

## 2. `quality_critique`

**場所**: `worker/pkg/worker/tools/process/critique.go`

### 動作

`GenerateStructuredSimple` で `CritiqueResult` スキーマを指定して評価させる。

```
SystemPrompt:
  "You are a QA reviewer. Evaluate the content for:
   - Hallucinations (claims not grounded in source)
   - Logical gaps or missing context
   - Inaccurate or misleading descriptions
   - Structural inconsistencies (broken parent-child relationships)
   Return valid=true only if no significant issues found."

UserPrompt:
  "Criteria: {criteria}\n\nContent to evaluate:\n{target_data}"
```

**失敗時**: `{valid: true}` を返して処理を継続させる（critique の失敗で persist が止まるべきではないため）。

---

## 3. `deduplicate_and_merge`

**場所**: `worker/pkg/worker/tools/process/merging.go`

### 引数の変更

旧設計の `item_ids []string` から `items []MergeCandidate` に拡張。LLM が判断するにはラベルと説明が必要なため。

```go
type MergeCandidate struct {
    LocalID     string `json:"local_id"`
    Label       string `json:"label"`
    Description string `json:"description"`
}
```

### 動作

```
SystemPrompt:
  "You are a knowledge deduplication expert. Select the single most comprehensive
   and accurate item as canonical. Return its local_id and reason."

UserPrompt:
  "[item_1] Label A: description...\n[item_2] Label B: description..."
```

**短絡**: Items が 0 件・1 件の場合は LLM を呼ばず即座に返す。

**失敗時**: 最初のアイテムを canonical として返す。

---

## 4. `goal_driven_synthesis`

**場所**: `worker/pkg/worker/tools/process/synthesis.go`

### 動作

チャンク群を一括で渡し、`[]SynthesizedItem`（階層構造付き）を構造化出力させる。Brief・Glossary は Working Memory 経由で system prompt に既に注入されているため引数に含めない。

```
SystemPrompt:
  "You are a knowledge architect. Convert document chunks into a hierarchical knowledge tree.
   Rules:
   - Use parent_local_id to express parent-child relationships. Root=empty.
   - Assign local_id as 'item_1', 'item_2', etc.
   - level: 1 for root-level, 2 for children, 3 for grandchildren.
   - description: grounded in source text, no hallucination.
   - summary_html: wrap in <p> tags, use <strong> for key terms.
   - source_chunk_ids: list referenced chunk IDs.
   - The document brief and glossary are in your system context — use them."

UserPrompt:
  "document_id: {id}\nInstruction: {instruction}\n\nChunks:\n[0] Heading\ntext..."
```

**失敗時**: 旧来の deterministic フォールバック（chunk のテキストをそのままコピー、level=1 フラット構造）を使う。LLM が落ちても処理は完走する。

---

## フォールバック設計の方針

| ツール | LLM 失敗時の振る舞い | 理由 |
|---|---|---|
| `generate_brief` | 最小限の Brief を生成して継続 | Brief がなくても synthesis は動く |
| `quality_critique` | `valid: true` を返して継続 | critique 失敗で persist を止めるべきでない |
| `deduplicate_and_merge` | 最初のアイテムを返して継続 | マージは品質向上であり必須処理ではない |
| `goal_driven_synthesis` | deterministic synthesis にフォールバック | ナレッジツリーは必ず生成する |
