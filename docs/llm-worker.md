# LLM Worker 設計仕様

現在の AI 実装の詳細は [docs/llm-worker-ai.md](/home/unix/Synthify/docs/llm-worker-ai.md) を参照。  
governance と権限モデルは [docs/llm-worker-governance.md](/home/unix/Synthify/docs/llm-worker-governance.md)、capability payload の実装仕様は [docs/llm-worker-capability-spec.md](/home/unix/Synthify/docs/llm-worker-capability-spec.md) を参照。

---

## 1. 方針

現在の worker は `fixed pass pipeline` ではなく、`job capability` に制約された `goal-driven execution` を前提にする。

- API は job を作成し、worker に `ExecuteApprovedPlan` を投げる
- worker は job に紐づく capability をロードして実行する
- node / edge / summary の mutation は capability 経由のみで許可する
- stage pipeline は実行手段であって、権限モデルの中心ではない

旧 2-pass synthesis は廃止し、現在の synthesis は `goal_driven_synthesis` に一本化している。

---

## 2. 実行フロー

```txt
API Service
  -> CreateProcessingJob
  -> issue default capability + execution plan
  -> enqueue worker request

Worker Service
  -> ExecuteApprovedPlan
  -> load job + capability
  -> run pipeline stages
  -> persist graph mutations through capability-aware repository
  -> mark plan/job complete
```

ローカル開発では API から Worker に直接 HTTP 呼び出しを行い、Cloud Tasks をバイパスする。

---

## 3. パイプライン

現在の標準 stage は以下。

1. `raw_intake`
2. `normalization`
3. `text_extraction`
4. `semantic_chunking`
5. `brief_generation`
6. `goal_driven_synthesis`
7. `persistence`
8. `html_summary_generation`

`goal_driven_synthesis` は chunk と brief を入力に、必要な node / edge 候補を直接組み立てる。  
LLM が失敗した場合は heuristic fallback に落とす。

---

## 4. PipelineContext

主要フィールドは以下。

```go
type PipelineContext struct {
    JobID       string
    DocumentID  string
    WorkspaceID string
    GraphID     string

    FileURI     string
    SourceFiles []SourceFile

    RawText string

    Chunks  []Chunk
    Outline []string

    DocumentBrief *DocumentBrief
    SectionBriefs []SectionBrief

    SynthesizedNodes []SynthesizedNode
    SynthesizedEdges []SynthesizedEdge

    NodeIDMap  map[string]string
    Capability *domain.JobCapability
}
```

worker は `Capability` が無い状態では mutation stage に進まない。

---

## 5. 権限境界

repository 境界で以下を強制する。

- `CreateStructuredNodeWithCapability`
- `CreateEdgeWithCapability`
- `UpdateNodeSummaryHTMLWithCapability`

チェック対象:

- job capability の存在
- graph / document / node のスコープ一致
- allowed operation
- mutation budget
- mutation log 記録

旧 direct write path は削除済みで、worker から無権限 mutation はできない。

---

## 6. Firestore Progress

Firestore は通知専用 projection であり、graph の正本ではない。

- `queued`
- `running`
- `completed`
- `failed`

`currentStage` には stage 名をそのまま入れる。  
現在の表示候補は `raw_intake` から `html_summary_generation` までの 8 stage。

---

## 7. 実装の正本

主要な参照先:

- [worker.proto](/home/unix/Synthify/proto/synthify/graph/v1/worker.proto)
- [worker.pb.go](/home/unix/Synthify/shared/gen/synthify/graph/v1/worker.pb.go)
- [processor.go](/home/unix/Synthify/worker/pkg/worker/processor.go)
- [goal_driven_synthesis.go](/home/unix/Synthify/worker/pkg/worker/stages/goal_driven_synthesis.go)
- [persistence.go](/home/unix/Synthify/worker/pkg/worker/stages/persistence.go)
- [html_summary_generation.go](/home/unix/Synthify/worker/pkg/worker/stages/html_summary_generation.go)
- [document.go](/home/unix/Synthify/shared/repository/postgres/document.go)
- [node.go](/home/unix/Synthify/shared/repository/postgres/node.go)

---

## 8. 今後の拡張

未着手または一部のみの項目:

- `GenerateExecutionPlan` と `EvaluateJobArtifact` の transport 実装
- capability / plan / approval の API read model 整備
- monitoring 指標の evaluator 中心化
- proto / generated code の正式再生成
