# API 層の仕様変更残骸とリファクタリング候補

## 背景

認証・アップロード・workspace membership・Firestore presence・LLM worker job 周りの仕様変更後、API / proto / mapper / service に古い前提が残っている。
このメモは、現時点で確認できた API 層の整理候補を優先度付きでまとめる。

mock repository の ID 生成や挙動はこのメモの対象外。

---

## P1 — 挙動不整合・バグ候補

### 1. `CreateProcessingJob` に `tree.TreeID` を `workspaceID` 引数として渡している

**場所**

- `api/internal/service/document.go` — `StartProcessing`
- `api/internal/service/document.go` — `ResumeProcessing`

**現状**

```go
tree, err := s.tree.GetOrCreateTree(ctx, wsID)
...
job := s.repo.CreateProcessingJob(ctx, documentID, tree.TreeID, jobType)
```

`CreateProcessingJob` の repository interface は以下。

```go
CreateProcessingJob(ctx context.Context, docID, workspaceID string, jobType treev1.JobType) *domain.DocumentProcessingJob
```

つまり第 2 引数は `workspaceID` だが、呼び出し側は `tree.TreeID` を渡している。

**補足**

現在の Postgres 実装では `GetOrCreateTree(ctx, wsID)` が `TreeID: wsID` を返しているため、実害が表面化しにくい。
ただし domain 上は `TreeID` と `WorkspaceID` は別フィールドであり、将来 tree ID が workspace ID から分離された瞬間に `document_processing_jobs.workspace_id` に tree ID が入る。

**修正方針**

呼び出しは `wsID` を渡す。

```go
job := s.repo.CreateProcessingJob(ctx, documentID, wsID, jobType)
```

そのうえで `ExecutePlanRequest.TreeID` には従来通り `tree.TreeID` を渡す。

---

### 2. `Document.updated_at` が実データを持っていない

**場所**

- `shared/mappers/mappers.go` — `ToProtoDocument`
- `shared/domain/types.go` — `Document`
- `proto/synthify/tree/v1/document.proto` — `Document.updated_at`

**現状**

```go
return &treev1.Document{
    ...
    CreatedAt: doc.CreatedAt,
    UpdatedAt: doc.CreatedAt,
}
```

proto の `Document` には `updated_at` があるが、domain `Document` には `UpdatedAt` がない。
そのため mapper は `CreatedAt` を流用している。

**問題**

API consumer から見ると `updated_at` が存在するのに、実際には更新時刻ではない。
document の処理状態や metadata 更新を `updated_at` で追う実装を作ると誤る。

**修正方針**

どちらかに寄せる。

- DB / domain に `documents.updated_at` を追加して mapper で返す
- まだ document 更新時刻を持たない仕様なら、proto から `updated_at` を削除するか deprecated 扱いにする

---

### 3. `Job.completed_at` が実完了時刻ではない

**場所**

- `shared/mappers/mappers.go` — `ToProtoJob`
- `shared/domain/types.go` — `DocumentProcessingJob`
- `proto/synthify/tree/v1/job.proto` — `Job.completed_at`

**現状**

```go
return &treev1.Job{
    ...
    CreatedAt:   job.CreatedAt,
    CompletedAt: job.UpdatedAt,
}
```

domain `DocumentProcessingJob` は `UpdatedAt` を持つが、`StartedAt` / `CompletedAt` は持たない。
mapper は `updated_at` を `completed_at` として返している。

**問題**

running / failed / succeeded のどの job でも、最終更新時刻が完了時刻として見える。
UI の処理時間表示や audit 表示が誤る可能性がある。

**修正方針**

どちらかに寄せる。

- DB / domain に `started_at` / `completed_at` を追加し、job lifecycle 更新時に明示的に書く
- 完了時刻をまだ扱わないなら、proto の `completed_at` を unset にする

---

## P2 — 重複コード・削除候補

### 4. `StartProcessing` と `ResumeProcessing` の dispatch ロジックが重複している

**場所**

- `api/internal/service/document.go`

**現状**

両メソッドに以下がほぼ同じ形で存在する。

- `GetOrCreateTree`
- `CreateProcessingJob`
- Firestore notifier `Queued`
- `ExecutePlanRequest` 作成
- `GenerateExecutionPlan`
- `ExecuteApprovedPlan`
- approval / rejected / failed 時の分岐
- `GetLatestProcessingJob` で再取得

**問題**

job dispatch の仕様変更時に片方だけ修正されるリスクが高い。
実際に `CreateProcessingJob(ctx, documentID, tree.TreeID, ...)` のような引数不整合が 2 箇所に重複している。

**修正方針**

private helper に抽出する。

```go
func (s *DocumentService) dispatchProcessingJob(
    ctx context.Context,
    wsID string,
    doc *domain.Document,
    jobType treev1.JobType,
) (*domain.DocumentProcessingJob, error)
```

`StartProcessing` は job type の決定だけ、`ResumeProcessing` は既存 running / queued job の再利用判定だけを持つ。

---

### 5. `GetUploadURL` RPC が現行 upload flow と二重化している

**場所**

- `proto/synthify/tree/v1/document.proto`
- `api/internal/handler/document.go` — `GetUploadURL`
- generated client/server code

**現状**

`CreateDocument` がすでに以下を返している。

```proto
message CreateDocumentResponse {
  Document document = 1;
  string upload_url = 2;
  string upload_method = 3;
  string upload_content_type = 4;
}
```

一方で `GetUploadURL` は別の tokenized path を作る古い設計のまま残っている。

```go
token := fmt.Sprintf("upload-%s", req.Msg.GetFilename())
uploadURL := h.uploadURLGenerator(req.Msg.GetWorkspaceId(), token+"/"+req.Msg.GetFilename())
```

web の handwritten code では `createDocument` / `startProcessing` が使われており、`getUploadURL` は generated client に存在するだけに見える。

**修正方針**

現行 flow を `CreateDocument -> upload_url PUT -> StartProcessing` に統一するなら、以下を削除する。

- `DocumentService.GetUploadURL`
- `GetUploadURLRequest`
- `GetUploadURLResponse`
- `DocumentHandler.GetUploadURL`
- `DocumentHandler.uploadURLGenerator` field が不要になるか確認

互換性が必要なら、deprecated コメントを proto に追加し、新規利用を止める。

---

### 6. Workspace membership RPC が account-level 管理に移行済み

**場所**

- `proto/synthify/tree/v1/workspace.proto`
- `api/internal/handler/workspace.go`

**現状**

以下 4 RPC はすべて `CodeUnimplemented` を返している。

- `InviteMember`
- `UpdateMemberRole`
- `RemoveMember`
- `TransferOwnership`

```go
return nil, connect.NewError(connect.CodeUnimplemented, errors.New("workspace membership is managed at account level"))
```

**問題**

proto に workspace-level membership API が残っているため、frontend / client から見ると使える API に見える。
実装は account-level に移管済みで、API contract と実態がずれている。

**修正方針**

account-level membership に統一済みなら、workspace proto から削除する。
移行互換が必要なら deprecated コメントを追加し、account-level API の正式な置き場所を定義する。

---

### 7. Item activity RPC が Firestore presence に移行済み

**場所**

- `proto/synthify/tree/v1/item.proto`
- `api/internal/handler/item.go`

**現状**

`RecordItemView` は空レスポンスを返すだけ。

```go
// Presence is managed in Firestore, so the backend does not write to Postgres here.
return connect.NewResponse(&treev1.RecordItemViewResponse{}), nil
```

`GetUserItemActivity` も空の `UserItemActivity` を返すだけ。

```go
// Item activity has already been moved to Firestore presence.
return connect.NewResponse(&treev1.GetUserItemActivityResponse{
    Activity: &treev1.UserItemActivity{},
}), nil
```

**問題**

API consumer から見ると activity API が使えそうに見えるが、実際には no-op / empty response。

**修正方針**

Firestore presence に完全移行済みなら proto / handler から削除する。
互換性が必要なら deprecated コメントを追加し、Firestore 側の購読 contract を docs に明記する。

---

### 8. `DocumentService.GetLatestProcessingJob` は service public method として不要そう

**場所**

- `api/internal/service/document.go`

**現状**

`DocumentService.GetLatestProcessingJob` は repository の薄い wrapper。
handler からの直接利用は見当たらず、`StartProcessing` / `ResumeProcessing` 内では repository を直接呼んでいる。

**修正方針**

外部利用がなければ削除する。
必要なら handler 経由で公開する用途を明確化し、service 内の job reload も helper に寄せる。

---

## P3 — 型・契約の整理

### 9. `domain.TreeItem` が未使用に見える

**場所**

- `shared/domain/types.go`

**現状**

`domain.TreeItem` は API item representation としてコメントされているが、現行 mapper / service は主に `domain.Item` / `domain.SubtreeItem` / proto `Item` を使っている。

**修正方針**

参照がないことを確認したうえで削除する。
もし将来の read model として残すなら、どの API が返す型かを明記する。

---

---

## P4 — 余計なラッパー関数・不要な間接レイヤー

**対応済み**

- `JobService` と `TreeService` は削除し、handler が repository を直接呼ぶ構成に変更済み。
- `ItemService` は `CreateItem` のみ残し、`GetTreeEntityDetail` / `ApproveAlias` / `RejectAlias` は handler から repository を直接呼ぶ構成に変更済み。
- `WorkspaceService.ListWorkspaces` は削除し、handler が `WorkspaceRepository.ListWorkspacesByUser` を直接呼ぶ構成に変更済み。
- `Worker.processDocument` と `Orchestrator.Agent()` の 1 行 wrapper は削除済み。

以下は「何も付加しない通過レイヤー」であり、削除または直接呼び出しに置き換えられる。

### 10. `JobService` 全体が薄いラッパー

**場所**: `api/internal/service/job.go`

全8メソッドが `repo.Xxx → ok → ErrNotFound` というだけのパターン。
`handlerutil.ToConnectError` が `domain.ErrNotFound → CodeNotFound` に変換するので、
handler が repo を直接呼べば service 層は何も付加しない。

**Before**

```go
// service/job.go
func (s *JobService) GetJob(ctx context.Context, jobID string) (*domain.DocumentProcessingJob, error) {
    job, ok := s.repo.GetProcessingJob(ctx, jobID)
    if !ok {
        return nil, domain.ErrNotFound
    }
    return job, nil
}
// GetExecutionPlan / ListApprovalRequests / ListMutationLogs /
// ListAllJobs / RequestApproval / ApproveApproval / RejectApproval も同型

// handler/job.go
job, err := h.service.GetJob(ctx, jobID)
if err != nil {
    return nil, handlerutil.ToConnectError(err)
}
```

**After**

```go
// service/job.go を削除
// handler/job.go が repo を直接持つ
type JobHandler struct {
    repo       repository.DocumentRepository
    workspaces repository.WorkspaceRepository
    documents  repository.DocumentRepository
}

job, ok := h.repo.GetProcessingJob(ctx, jobID)
if !ok {
    return nil, connect.NewError(connect.CodeNotFound, domain.ErrNotFound)
}
```

---

### 11. `TreeService` 全体が薄いラッパー

**場所**: `api/internal/service/tree.go`

全4メソッドが repo を呼ぶだけで、型変換すら行わない。

**Before**

```go
func (s *TreeService) GetTreeByWorkspace(ctx context.Context, workspaceID string) ([]*domain.Item, error) {
    items, ok := s.repo.GetTreeByWorkspace(ctx, workspaceID)
    if !ok { return nil, domain.ErrNotFound }
    return items, nil
}
func (s *TreeService) GetOrCreateTree(ctx context.Context, workspaceID string) (*domain.Tree, error) {
    return s.repo.GetOrCreateTree(ctx, workspaceID)  // そのまま転送
}
func (s *TreeService) FindPaths(...) ([]*domain.Item, []domain.TreePath, error) {
    items, paths, ok := s.repo.FindPaths(...)
    if !ok { return nil, nil, domain.ErrNotFound }
    return items, paths, nil
}
func (s *TreeService) GetSubtree(...) ([]*domain.SubtreeItem, error) {
    return s.repo.GetSubtree(...)  // そのまま転送
}
```

**After**

```go
// service/tree.go を削除
// handler/tree.go が TreeRepository を直接持つ
type TreeHandler struct {
    repo       repository.TreeRepository
    workspaces repository.WorkspaceRepository
    documents  repository.DocumentRepository
}
```

---

### 12. `ItemService` の3メソッドがラッパー

**場所**: `api/internal/service/item.go`

`GetTreeEntityDetail`、`ApproveAlias`、`RejectAlias` の3つが repo への直通。
`CreateItem` は `GetOrCreateTree` を事前に呼ぶため残す価値がある。

**Before**

```go
func (s *ItemService) GetTreeEntityDetail(ctx context.Context, itemID string) (*domain.Item, error) {
    item, ok := s.repo.GetItem(ctx, itemID)
    if !ok { return nil, domain.ErrNotFound }
    return item, nil
}
func (s *ItemService) ApproveAlias(ctx context.Context, wsID, canonical, alias string) error {
    if !s.repo.ApproveAlias(ctx, wsID, canonical, alias) { return domain.ErrNotFound }
    return nil
}
// RejectAlias も同型
```

**After**

```go
// この3メソッドを service/item.go から削除
// handler/item.go が ItemRepository を直接呼ぶ
item, ok := h.items.GetItem(ctx, req.Msg.GetTargetRef().GetId())
if !ok {
    return nil, connect.NewError(connect.CodeNotFound, domain.ErrNotFound)
}
```

---

### 13. `WorkspaceService.ListWorkspaces` がラッパー

**場所**: `api/internal/service/workspace.go:20-22`

`GetWorkspace`（認可チェックあり）・`CreateWorkspace`（Account取得あり）は意味があるが、
`ListWorkspaces` だけは完全な1行ラッパー。

**Before**

```go
func (s *WorkspaceService) ListWorkspaces(ctx context.Context, userID string) []*domain.Workspace {
    return s.workspaces.ListWorkspacesByUser(ctx, userID)
}
```

**After**

```go
// service から削除し、handler が直接呼ぶ
func (h *WorkspaceHandler) ListWorkspaces(ctx context.Context, _ *connect.Request[...]) (*connect.Response[...], error) {
    user, err := currentUser(ctx)
    if err != nil { return nil, err }
    workspaces := h.workspaceRepo.ListWorkspacesByUser(ctx, user.ID)
    ...
}
```

---

### 14. `Worker.processDocument` が1行ラッパー

**場所**: `worker/pkg/worker/worker.go:138-140`

```go
// Before
func (w *Worker) processDocument(ctx context.Context, req ExecutePlanRequest) error {
    return w.orchestrator.ProcessDocument(ctx, w.runner,
        req.JobID, req.DocumentID, req.WorkspaceID, req.FileURI, req.Filename, req.MimeType)
}
// Process 内: if err := w.processDocument(ctx, req); err != nil {

// After: processDocument を削除し Process 内で直接呼ぶ
if err := w.orchestrator.ProcessDocument(ctx, w.runner,
    req.JobID, req.DocumentID, req.WorkspaceID, req.FileURI, req.Filename, req.MimeType); err != nil {
```

---

### 15. `Orchestrator.Agent()` がフィールドを返すだけのゲッター

**場所**: `worker/pkg/worker/agents/orchestrator.go:191-196`

`worker.go` の `NewWorkerWithNotifier` 1箇所でのみ使われる。

**Before**

```go
// orchestrator.go
func (o *Orchestrator) Agent() agent.Agent {
    if o == nil { return nil }
    return o.agent
}

// worker.go
r, err := runner.New(runner.Config{
    Agent: orch.Agent(),
    ...
})
```

**After**

```go
// ゲッターを削除し agent を公開フィールドにするか、
// NewOrchestrator 内でそのまま runner.Config に渡す構造にする
r, err := runner.New(runner.Config{
    Agent: orch.agent,
    ...
})
```

---

### 16. `worker/pkg/worker/pipeline` パッケージが旧アーキテクチャの残骸

**場所**: `worker/pkg/worker/pipeline/` (runner.go, context.go, stage.go, jobstatus.go)

ADK エージェント移行後、このパッケージは **一切インポートされていない**（`grep` で確認済み）。

`Worker.Process` が `PipelineRunner.Run` と全く同じライフサイクル管理を手動で再実装しており、
`PipelineRunner` の存在意義がない。

**PipelineRunner.Run（使われていない）**

```go
func (r *PipelineRunner) Run(ctx context.Context, pctx *PipelineContext) error {
    r.jobRepo.MarkProcessingJobRunning(pctx.JobID)
    if r.notifier != nil { r.notifier.Running(ctx, pctx.JobStatusPayload()) }
    for _, stage := range r.stages { ... }
    r.jobRepo.CompleteProcessingJob(pctx.JobID)
    if r.notifier != nil { r.notifier.Completed(ctx, pctx.JobStatusPayload()) }
    return nil
}
```

**Worker.Process（同じことを手動で実装している）**

```go
w.repo.MarkProcessingJobRunning(ctx, req.JobID)
if w.status != nil { w.status.Running(ctx, payload) }
if err := w.orchestrator.ProcessDocument(...); err != nil {
    w.repo.FailProcessingJob(ctx, req.JobID, err.Error())
    if w.status != nil { w.status.Failed(ctx, payload, err.Error()) }
    return err
}
w.repo.CompleteProcessingJob(ctx, req.JobID)
if w.status != nil { w.status.Completed(ctx, payload) }
```

`PipelineContext` の `RawText`, `Chunks`, `DocumentBrief`, `SynthesizedItems` 等のフィールドも
旧パイプラインアーキテクチャの中間状態で、現行の ADK ツール設計では使われていない。
ステージ定数（`StageRawIntake`, `StageSemanticChunking` 等）も同様。

**After**

```
worker/pkg/worker/pipeline/ ディレクトリごと削除
```

---

## 実装順の提案

1. `CreateProcessingJob` 呼び出しを `tree.TreeID` から `wsID` に修正し、テストを追加する（バグ修正）
2. `Document.updated_at` と `Job.completed_at` の contract を決める（proto 設計）
3. `StartProcessing` / `ResumeProcessing` の dispatch helper 抽出
4. `worker/pkg/worker/pipeline/` ディレクトリ削除（依存なしで安全）
5. `Worker.processDocument` / `Orchestrator.Agent()` の1行ラッパー削除（対応済み）
6. `GetUploadURL` / workspace membership / item activity RPC を削除または deprecated 化する
7. service 層の薄いラッパー（JobService, TreeService, ItemService 一部）を handler に統合する（対応済み）
8. 未使用 domain 型（`TreeItem`）を削除する
