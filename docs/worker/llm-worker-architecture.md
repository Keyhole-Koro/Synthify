# LLM Worker アーキテクチャ

## 基本思想：LLM はオーケストレーター

Worker の中心にいるのは LLM エージェントであり、コードではない。

```
Process(req)
  └─ orchestrator.ProcessDocument(job_id, document_id, ...)
       └─ LLM が自律的にツールを呼ぶ
            ├─ extract_text
            ├─ semantic_chunking
            ├─ generate_brief        → Working Memory に書き込む
            ├─ goal_driven_synthesis
            ├─ quality_critique
            └─ persist_knowledge_tree
```

コードがやることは「ジョブのバリデーションと LLM の起動」だけ。処理の順序・判断・品質管理は LLM に委ねる。

`Orchestrator` はシステム内で LLM を起動する唯一の場所であり、すべての LLM 呼び出しはここを経由する。`process/` の各ツールが内部で `base.LLMClient` を呼ぶ場合も、それらは ADK エージェントループから呼ばれているため、起点は常に `Orchestrator.ProcessDocument` になる。

---

## Orchestrator と jobID 管理

```go
type Orchestrator struct {
    agent        agent.Agent
    currentJobID atomic.Pointer[string]   // 実行中のジョブIDをスレッドセーフに保持
}
```

`currentJobID` は2つの場所で使われる。

**書き込み — `ProcessDocument` の冒頭**

```go
o.currentJobID.Store(&jobID)
```

新しいジョブが始まるたびに最新の jobID を記録する。

**読み出し — `AfterToolCallback`**

```go
if p := orch.currentJobID.Load(); p != nil && *p != "" {
    _ = logger.LogToolCall(ctx, *p, t.Name(), argJSON, resJSON, durationMs)
}
```

ツールが呼ばれるたびに「ジョブID・ツール名・引数・結果・実行時間」を DB に記録する。

**なぜ `atomic.Pointer` か**

`AfterToolCallback` は ADK ランタイムのゴルーチンから呼ばれる可能性がある。`jobID` を普通のフィールドに入れると data race になるため、アトミック操作で保護している。また `Orchestrator` が複数ジョブにわたって再利用される設計のため、クロージャに `jobID` を直接キャプチャせず、フィールドを経由して常に最新値を参照する。

---

## ツールの 3 層構造

ツールは役割によって 3 つのパッケージに分かれている。

```
worker/pkg/worker/tools/
├── base/     共有基盤（インターフェース・依存性・ユーティリティ）
├── memory/   Working Memory（LLM のコンテキストに自動注入される状態）
├── process/  処理系（LLM に推論・判断をさせるツール）
└── io/       I/O 系（データの読み書き・変換。LLM を必要としない）
```

### base/ — 共有基盤

全ツールが参照する型と依存性を置く場所。

- `Context` — `Repo`、`Embedder`、`Memories` を持つ依存性コンテナ
- `PromptMemory` — Working Memory に自動注入されるブロックのインターフェース
- `RenderWorkingMemory()` — 全 PromptMemory を結合して system prompt に差し込む文字列を返す

### memory/ — Working Memory

**「LLM が能動的に読みに行く必要がない情報」** を置く層。

毎ターンの `BeforeModelCallback` で system prompt に自動注入される。LLM は常に最新の状態を参照できる。

| 型 | 内容 | 更新タイミング |
|---|---|---|
| `Brief` | ドキュメントの主題・要約 | `generate_brief` ツール呼び出し時 |
| `Glossary` | 用語定義 | `glossary_register` ツール呼び出し時 |
| `Journal` | タスクリスト | `journal_add_task` / `journal_update_task` 時 |

**読み出しはツールにしない。** `manage_glossary(action=list)` のような読み出し専用アクションは存在しない。LLM が呼ぶかどうかに依存しない自動注入が信頼性を担保する。

**書き込みだけがツールになる。** LLM が意図的に状態を更新するときだけツールを呼ぶ。

### process/ — 処理系ツール

LLM に推論・変換をさせるツール群。引数はツール固有の「操作の入力」だけを持つ。

| ツール | 入力 | 出力 |
|---|---|---|
| `generate_brief` | outline | DocumentBrief（→ Brief に書き込む） |
| `goal_driven_synthesis` | chunks, instruction | []SynthesizedItem |
| `quality_critique` | target_data, criteria | CritiqueResult |
| `deduplicate_and_merge` | items[] | MergedID |
| `generate_html_summary` | item | HTML |

**Brief や Glossary は引数に含めない。** これらは Working Memory 経由で system prompt に既に注入されている。ツールのスキーマは「操作の引数」だけを表現する。

### io/ — I/O 系ツール

ドキュメントの入出力・構造抽出を行うツール群。LLM を使わず確定的に動作する。

| ツール | 役割 |
|---|---|
| `extract_text` | URI からテキストを取得 |
| `semantic_chunking` | テキストをチャンクに分割・ベクトル保存 |
| `semantic_search` | ベクトル検索で関連チャンクを取得 |
| `persist_knowledge_tree` | アイテムを DB に書き込む |
| `analyze_dependencies` | アウトラインから処理順を解析 |
| `extract_table_data` | テーブルを JSON 化 |
| `repair_encoding` | 文字化けを修復 |

---

## ツールのスキーマ設計原則

**プロンプトに注入される情報はスキーマに含めない。**

Working Memory（Brief、Glossary、Journal）はすでに system prompt にある。LLM がツールを呼ぶとき、これらを引数として再度渡す必要はない。

```
❌ 旧設計
goal_driven_synthesis(chunks, document_brief, glossary[], parent_structure)
                                ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                                Working Memory と二重になっていた

✅ 現在の設計
goal_driven_synthesis(chunks, instruction)
```

ツールのスキーマが小さいほど、LLM が正しく呼び出せる確率が上がる。

---

## Working Memory の注入メカニズム

```
毎ターン BeforeModelCallback が発火
  └─ base.Context.RenderWorkingMemory()
       ├─ Brief.RenderForPrompt()    → "### Document Brief\n..."
       ├─ Glossary.RenderForPrompt() → "### Glossary\n..."
       └─ Journal.RenderForPrompt()  → "### Tasks\n..."
  └─ genai.GenerateContentConfig.SystemInstruction に追記
```

LLM は毎ターン最新の Working Memory を持った状態でツール選択を行う。

---

## ADK の ReAct ループ

`Orchestrator.ProcessDocument` が `runner.Run()` を呼ぶと、ADK 内部の `Flow.Run` が **`for { runOneStep() }` の無限ループ**として動く。

```
Flow.Run()
  for {
    runOneStep()
      ├─ BeforeModelCallback  ← Working Memory をここで注入
      ├─ LLM 呼び出し
      ├─ ツール呼び出し（並列）
      └─ AfterToolCallback    ← ツールログをここで記録
    lastEvent.IsFinalResponse() == true → return
                              == false → 次のループへ
  }
```

ループは `IsFinalResponse()` が `true` になるまで続く。コード側は何も管理しない。

### IsFinalResponse の判定ロジック

```go
// session/session.go
func (e *Event) IsFinalResponse() bool {
    if e.Actions.SkipSummarization || len(e.LongRunningToolIDs) > 0 {
        return true
    }
    return !hasFunctionCalls(&e.LLMResponse) &&
           !hasFunctionResponses(&e.LLMResponse) &&
           !e.LLMResponse.Partial &&
           !hasTrailingCodeExecutionResult(&e.LLMResponse)
}
```

実質的な終了条件は **「LLM がテキストだけ返してきた」**。ツールを1つでも呼んだターンは `hasFunctionCalls = true` になるため `false` となり、ツール結果を会話履歴に追加して再度 LLM に投げる。

| イベントの内容 | IsFinalResponse |
|---|---|
| LLM がテキストのみ返した | `true` → ループ終了 |
| LLM がツール呼び出しを含む | `false` → ループ継続 |
| ツール結果を処理中 | `false` → ループ継続 |
| ストリーミング途中（Partial） | `false` → ループ継続 |

---

## コードと LLM の責任分界

| 責任 | 担当 |
|---|---|
| ジョブのバリデーション | コード（`worker.go`） |
| ジョブのステータス管理 | コード（`worker.go`） |
| ツールの実行（I/O） | コード（`io/` パッケージ） |
| Working Memory の注入 | コード（`BeforeModelCallback`） |
| 処理順序の決定 | LLM |
| 内容の推論・構造化 | LLM（`process/` パッケージのツール経由） |
| 品質の判断 | LLM（`quality_critique` 経由） |
