# LLM Worker Capability 実装仕様

## 目的

`JobCapability` は、LLM worker が 1 つの processing job で実行してよい操作と使用量を制限するための権限トークンである。

worker はバックグラウンドで LLM と tool を動かすため、agent が意図せず広い mutation を行ったり、LLM / tool / item 作成を無制限に消費したりしないよう、job ごとに capability を発行して実行時に検査する。

## 保存場所

DB の正本は `job_capabilities`。

```sql
CREATE TABLE IF NOT EXISTS job_capabilities (
  capability_id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  allowed_document_ids_json TEXT NOT NULL DEFAULT '[]',
  allowed_item_ids_json TEXT NOT NULL DEFAULT '[]',
  allowed_operations_json TEXT NOT NULL DEFAULT '[]',
  max_llm_calls INTEGER NOT NULL DEFAULT 0,
  max_tool_runs INTEGER NOT NULL DEFAULT 0,
  max_item_creations INTEGER NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);
```

Go の domain 型は `shared/domain/types.go` の `JobCapability`。

```go
type JobCapability struct {
    CapabilityID       string
    JobID              string
    WorkspaceID        string
    AllowedDocumentIDs []string
    AllowedItemIDs     []string
    AllowedOperations  []treev1.JobOperation
    MaxLLMCalls        int
    MaxToolRuns        int
    MaxItemCreations   int
    ExpiresAt          string
    CreatedAt          string
}
```

## デフォルト capability

`domain.DefaultJobCapability` は job 作成時の標準値を作る。

現状の主な値:

| field | default | 意味 |
|---|---:|---|
| `AllowedDocumentIDs` | current document only | job 対象 document だけを許可 |
| `AllowedOperations` | read tree/document, create/update item, invoke LLM | worker の標準処理に必要な操作 |
| `MaxLLMCalls` | `128` | ADK model call と tool 内 LLM call の上限 |
| `MaxToolRuns` | `0` | `0` は無制限 |
| `MaxItemCreations` | `4096` | 作成予定 item 数の上限 |
| `ExpiresAt` | created_at + 24h | capability の有効期限 |

`0` の上限値は unlimited と解釈する。

## 操作権限の検査

DB mutation 側の実検査は repository 層で行う。

代表例:

- `CreateStructuredItemWithCapability`
- `UpdateItemSummaryHTMLWithCapability`

これらは `canMutateTree` を通して以下を確認する。

- capability が nil でない
- operation が `AllowedOperations` に含まれる
- workspace が capability の workspace と一致する
- document が `AllowedDocumentIDs` に含まれる
- item 更新の場合は `AllowedItemIDs` の制約を満たす
- capability が期限切れでない

この層は最終防衛線であり、agent / tool が誤った引数を出しても DB mutation を拒否する。

## 使用量上限の検査

worker runtime 側の使用量は `worker/pkg/worker/tools/base/usage.go` の `UsageLimiter` が管理する。

`NewWorkerWithNotifier` で `UsageLimiter` を作成し、`base.Context` に接続する。

```go
usage := base.NewUsageLimiter(treeRepo)
b := &base.Context{
    Repo:  treeRepo,
    LLM:   base.NewCountingLLMClient(llmClient, usage),
    Usage: usage,
}
```

`ProcessDocument` 開始時に `BeginJob(ctx, jobID)` を呼び、DB から job capability を読み込んで counters を初期化する。

### LLM call

LLM call は 2 経路で数える。

| 経路 | 検査場所 |
|---|---|
| ADK orchestrator の model call | `BeforeModelCallbacks` |
| tool 内で `base.LLMClient` を直接呼ぶ call | `CountingLLMClient` |

これにより、`goal_driven_synthesis` / `generate_brief` / `quality_critique` などが `b.LLM.GenerateStructured` を呼ぶ場合も、tool 側に個別カウントを書かずに `MaxLLMCalls` を強制できる。

### Tool run

ADK tool run は `BeforeToolCallbacks` で一元カウントする。

tool 実装ごとに `IncrementToolRuns` を書かない。新しい tool を `Tools: []tool.Tool{...}` に追加すれば、agent 経由の tool call は callback を必ず通る。

注意: ADK を通らず Go code から tool を直接 `Run` する場合は、この callback を通らない。その場合は呼び出し元で別途 usage を通す必要がある。

### Item creation

item creation は `persist_knowledge_tree` の中で数える。

理由は、tool call は 1 回でも作成予定 item 数は `len(args.Items)` に依存するため。

```go
if err := b.IncrementItemCreations(ctx, len(args.Items)); err != nil {
    return PersistenceResult{}, err
}
```

現状は「作成予定数」を persist 前に加算する。途中で DB 作成が失敗してもカウントは戻さないため、retry 時には保守的に効く。

`IncrementItemCreations` は重複チェックをしない。`args.Items` に同じ概念の item が複数含まれている場合も、その件数分を作成予定数として budget に加算する。既存 `tree_items` と同一か、同じ `item_sources` が既にあるか、同じ label か、といった品質・冪等性の判定はこの関数の責務ではない。

重複排除や再処理時の二重作成防止は、別の層で扱う。

- synthesis 後の `deduplicate_and_merge`
- persistence 前の local item dedupe
- DB 保存時の deterministic id / unique key / upsert
- `item_sources` を使った「この document chunk 由来は保存済み」の判定
- snapshot / checkpoint による persistence 済み stage の skip

DB repository 側にも `CreateStructuredItemWithCapability` の `MaxItemCreations` 検査がある。runtime usage は早期停止、repository 検査は最終防衛線という位置づけ。

## エラー時の挙動

上限を超えると `UsageLimiter` は error を返す。

例:

```text
job job_123 exceeded LLM call limit: 129 > 128
job job_123 exceeded tool run limit: 51 > 50
job job_123 exceeded item creation limit: 4097 > 4096
```

この error は ADK callback または tool から worker に伝播し、`Worker.Process` が job を failed にする。

## 抜け道と注意点

`BeforeToolCallbacks` で抜け漏れをかなり減らせるが、以下は別途注意する。

- ADK agent を通らない deterministic pipeline
- tool を Go code から直接 `Run` するテストや補助処理
- `base.LLMClient` 以外の LLM client を直接呼ぶ code
- sub-agent / 別 runner を作り、同じ callbacks を設定しない場合
- retry wrapper の内側 / 外側でどちらを 1 call とみなすか

現在の `CountingLLMClient` は wrapper の入口で 1 回数える。retry が内部で複数 provider call を行っても、worker budget 上は「1 logical LLM request」として扱う。

## 関連ファイル

- `shared/domain/types.go` — `JobCapability` と `DefaultJobCapability`
- `db/init/001_schema.sql` — `job_capabilities`
- `shared/repository/postgres/document.go` — capability の保存・取得
- `shared/repository/postgres/item.go` — mutation 時の capability 検査
- `worker/pkg/worker/tools/base/usage.go` — runtime usage enforcement
- `worker/pkg/worker/agents/orchestrator.go` — ADK model/tool callbacks
- `worker/pkg/worker/tools/io/persistence.go` — item creation count
