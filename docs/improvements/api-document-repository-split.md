# PR 1: DocumentRepository を分割

親: [api-layering-cleanup.md](api-layering-cleanup.md)

## 現状の課題

`apps/api/internal/repository/interfaces.go` の `DocumentRepository` interface に **32 メソッド** が同居している。Documents 自体の操作の他に、Chunks / Files / Jobs / Job Approval / Job Logs / Tool Calls / Embeddings が全部入りになっており、Interface Segregation Principle に反する。

具体的に困っていること:
- mock を 1 つでも実装すると 32 メソッドのスタブが必要になる。
- handler / service が「自分は Document の何を使っているか」を引数の型から読み取れない。
- 変更影響範囲が広い。たとえば Job 関連のメソッドを足すと Document の全 mock を再生成しないといけない。

## 改善目標

利用側の使い方に沿って 5 つの interface に分割する。Postgres `Store` はそれらを全部満たす形を維持するので、cmd/server のワイヤリング側は基本そのまま (引数の型を変えるだけ)。

## 分割案

| 新 interface | 含むメソッド |
|---|---|
| `DocumentRepository` (狭く再定義) | `ListDocuments`, `GetDocument`, `CreateDocument`, `ConfirmDocumentUpload`, `ExpireUploadReservations`, `CreateDocumentFile`, `ListDocumentFiles`, `GetDocumentFileByPath` |
| `DocumentChunkRepository` | `GetDocumentChunks`, `SaveDocumentChunks`, `SearchRelatedChunksByVector` |
| `JobRepository` | `GetLatestProcessingJob`, `GetProcessingJob`, `GetJobCapability`, `GetJobExecutionPlan`, `UpsertJobExecutionPlan`, `UpsertJobEvaluation`, `EvaluateJob`, `CreateProcessingJob`, `MarkProcessingJobRunning`, `UpdateProcessingJobStage`, `FailProcessingJob`, `CompleteProcessingJob`, `ListAllJobs`, `GetJobPlanningSignals` |
| `JobApprovalRepository` | `ListJobApprovalRequests`, `RequestJobApproval`, `ApproveJobApproval`, `RejectJobApproval` |
| `JobLogRepository` | `ListJobMutationLogs`, `ListJobLogs`, `SearchJobLogs`, `ListRelatedJobLogs`, `LogToolCall` |

注: `GetJobPlanningSignals` は Document + Job + Tree の交差情報だが、利用側 (worker) が Job 系として扱っているので `JobRepository` に置く。

## 実装計画

### Phase 1: interface 定義の追加
- `apps/api/internal/repository/interfaces.go` に 5 つの interface を新規定義する。
- 旧 `DocumentRepository` は **当面残す** (移行期間の互換目的)。コメントで Deprecated を明示。

### Phase 2: 利用側の引数型を狭める
- 各 handler / service が `DocumentRepository` を受け取っている箇所を、実際に使っているメソッドの subset interface に差し替える。
  - 例: `JobHandler` は `JobRepository` + `JobApprovalRepository` + `JobLogRepository` を受け取る。
  - 例: `DocumentService` は `DocumentRepository` (新, 狭い) + 必要なら `JobRepository` を受け取る。
- コンストラクタ呼び出し側 (`cmd/server/main.go`) では同じ `*Store` を渡すだけで OK (Store が全 interface を満たすため)。

### Phase 3: 旧 DocumentRepository を削除
- Phase 2 で全利用箇所が新 interface に切り替わったことを確認したら、旧 `DocumentRepository` を削除する。
- mock store も新 interface 群を実装する形に書き直す (実装内容は同じ、グルーピングだけ変える)。

## 範囲外

- 既存メソッドのシグネチャ変更はしない (rename, パラメータ変更等)。今回は純粋な interface 分割のみ。
- `WorkspaceRepository`, `AccountRepository`, `UserRepository` 等の他 interface は触らない。

## 完了条件

- `go build ./apps/api/...` が通る。
- `go test ./apps/api/...` が pass する。
- 旧 `DocumentRepository` が repository 配下から削除されている。
- 各 handler / service のコンストラクタ引数の型が、実際に使うメソッドだけを持つ interface になっている。
