# LLM Worker Capability 実装仕様

この文書は [llm-worker-governance.md](/home/unix/Synthify/docs/llm-worker-governance.md) を、現在の Synthify コードベースに合わせて `proto / DB schema / API boundary` へ落とした実装仕様である。

ここでの目的は、現在の `ExecuteApprovedPlan` ベース worker に対して、capability / plan / approval の境界を明示することである。

- job ごとの capability 発行
- planner / worker / evaluator / governor の責務分離
- node mutation の監査可能化
- 高リスク操作に対する approval 境界

---

## 1. 前提

現状の実装では、worker は主に `document_processing_jobs` を起点に動く。

- proto: [job.proto](/home/unix/Synthify/proto/synthify/graph/v1/job.proto), [worker.proto](/home/unix/Synthify/proto/synthify/graph/v1/worker.proto), [tool.proto](/home/unix/Synthify/proto/synthify/graph/v1/tool.proto)
- schema: [001_schema.sql](/home/unix/Synthify/db/init/001_schema.sql)
- repository: [interfaces.go](/home/unix/Synthify/shared/repository/interfaces.go)

このため、新しい権限モデルも `job に紐づく capability token` を中心に設計する。

---

## 2. 導入方針

導入は 3 段階に分ける。

### Phase 1

最小差分で capability と監査ログを入れる。

- `document_processing_jobs` を拡張
- `job_capabilities`
- `job_execution_plans`
- `job_mutation_logs`
- `job_approval_requests`

この段階では planner / evaluator は内部実装でもよく、proto は内部 API 中心でよい。

### Phase 2

plan / approval / evaluation を明示的な API に切り出す。

- `JobService` に plan / approval 状態取得を追加
- `WorkerService` を plan 実行起点に寄せる
- node mutation を API / repository で capability 検査する

### Phase 3

human-curated node と locked field を導入し、worker の直接上書きを制限する。

---

## 3. Proto 変更案

### 3.1 `job.proto`

`Job` は status だけでは足りないので、capability / plan / approval 状態を持てるようにする。

追加すべき message:

```proto
message JobCapability {
  string capability_id = 1;
  string job_id = 2;
  string workspace_id = 3;
  string graph_id = 4;
  repeated string allowed_document_ids = 5;
  repeated string allowed_node_ids = 6;
  repeated JobOperation allowed_operations = 7;
  int32 max_llm_calls = 8;
  int32 max_tool_runs = 9;
  int32 max_node_creations = 10;
  int32 max_edge_mutations = 11;
  string expires_at = 12;
}

enum JobOperation {
  JOB_OPERATION_UNSPECIFIED = 0;
  JOB_OPERATION_READ_GRAPH = 1;
  JOB_OPERATION_READ_DOCUMENT = 2;
  JOB_OPERATION_CREATE_NODE = 3;
  JOB_OPERATION_UPDATE_NODE = 4;
  JOB_OPERATION_CREATE_EDGE = 5;
  JOB_OPERATION_DELETE_EDGE = 6;
  JOB_OPERATION_RUN_NORMALIZATION_TOOL_DRY_RUN = 7;
  JOB_OPERATION_RUN_NORMALIZATION_TOOL_APPLY = 8;
  JOB_OPERATION_INVOKE_LLM = 9;
  JOB_OPERATION_EMIT_PLAN = 10;
  JOB_OPERATION_EMIT_EVAL = 11;
}

message JobExecutionPlan {
  string plan_id = 1;
  string job_id = 2;
  PlanStatus status = 3;
  repeated JobPlanStep steps = 4;
  string summary = 5;
  string created_at = 6;
}

message JobPlanStep {
  string step_id = 1;
  string title = 2;
  string rationale = 3;
  JobRiskTier risk_tier = 4;
  repeated JobOperation operations = 5;
  repeated string target_node_ids = 6;
  repeated string target_document_ids = 7;
  string rollback_strategy = 8;
}

enum JobRiskTier {
  JOB_RISK_TIER_UNSPECIFIED = 0;
  JOB_RISK_TIER_0_READ_ONLY = 1;
  JOB_RISK_TIER_1_SAFE_MUTATION = 2;
  JOB_RISK_TIER_2_REVIEW_REQUIRED = 3;
  JOB_RISK_TIER_3_APPROVAL_REQUIRED = 4;
}

enum PlanStatus {
  PLAN_STATUS_UNSPECIFIED = 0;
  PLAN_STATUS_DRAFT = 1;
  PLAN_STATUS_APPROVED = 2;
  PLAN_STATUS_REJECTED = 3;
  PLAN_STATUS_EXECUTING = 4;
  PLAN_STATUS_COMPLETED = 5;
}

message ApprovalRequest {
  string approval_id = 1;
  string job_id = 2;
  string plan_id = 3;
  string reason = 4;
  JobRiskTier risk_tier = 5;
  repeated JobOperation requested_operations = 6;
  ApprovalStatus status = 7;
  string requested_at = 8;
  string reviewed_at = 9;
  string reviewed_by = 10;
}

enum ApprovalStatus {
  APPROVAL_STATUS_UNSPECIFIED = 0;
  APPROVAL_STATUS_PENDING = 1;
  APPROVAL_STATUS_APPROVED = 2;
  APPROVAL_STATUS_REJECTED = 3;
}
```

追加すべき RPC:

```proto
rpc GetJobCapability(GetJobCapabilityRequest) returns (GetJobCapabilityResponse);
rpc GetJobExecutionPlan(GetJobExecutionPlanRequest) returns (GetJobExecutionPlanResponse);
rpc ListJobApprovalRequests(ListJobApprovalRequestsRequest) returns (ListJobApprovalRequestsResponse);
rpc ApproveJobRequest(ApproveJobRequestRequest) returns (ApproveJobRequestResponse);
rpc RejectJobRequest(RejectJobRequestRequest) returns (RejectJobRequestResponse);
```

意図:

- `GetJobStatus` は軽量な進捗確認に残す
- capability / plan / approval は別 read model として切り出す

### 3.2 `worker.proto`

現在の internal worker RPC は `ExecuteApprovedPlan` を起点にしている。  
今後は planner / evaluator も別 RPC に切り出す。

推奨案:

```proto
rpc GenerateExecutionPlan(GenerateExecutionPlanRequest) returns (GenerateExecutionPlanResponse);
rpc ExecuteApprovedPlan(ExecuteApprovedPlanRequest) returns (ExecuteApprovedPlanResponse);
rpc EvaluateJobArtifact(EvaluateJobArtifactRequest) returns (EvaluateJobArtifactResponse);
```

追加 message:

```proto
message GenerateExecutionPlanRequest {
  string job_id = 1;
}

message GenerateExecutionPlanResponse {
  JobExecutionPlan plan = 1;
}

message ExecuteApprovedPlanRequest {
  string job_id = 1;
  string plan_id = 2;
  string capability_id = 3;
}

message ExecuteApprovedPlanResponse {
  string status = 1;
}

message EvaluateJobArtifactRequest {
  string job_id = 1;
  string plan_id = 2;
}

message EvaluationResult {
  bool passed = 1;
  string summary = 2;
  repeated string findings = 3;
  int32 score = 4;
}

message EvaluateJobArtifactResponse {
  EvaluationResult result = 1;
}
```

意図:

- planner, worker, evaluator を 1 RPC に混ぜない
- goal-driven な node 更新は `GenerateExecutionPlan -> approval -> ExecuteApprovedPlan -> EvaluateJobArtifact` に分離する

### 3.3 `graph_types.proto`

Node に governance field を足す余地を作る。

```proto
enum NodeGovernanceState {
  NODE_GOVERNANCE_STATE_UNSPECIFIED = 0;
  NODE_GOVERNANCE_STATE_SYSTEM_GENERATED = 1;
  NODE_GOVERNANCE_STATE_PENDING_REVIEW = 2;
  NODE_GOVERNANCE_STATE_HUMAN_CURATED = 3;
  NODE_GOVERNANCE_STATE_LOCKED = 4;
}

message Node {
  string id = 1;
  string label = 2;
  int32 level = 3;
  string description = 4;
  string summary_html = 5;
  string created_at = 6;
  GraphProjectionScope scope = 7;
  NodeGovernanceState governance_state = 8;
  bool worker_writable = 9;
}
```

これにより、UI と API の両方で `worker が触ってよい node か` を表現できる。

### 3.4 `tool.proto`

`RunNormalizationTool` は現状でも危険度が高いので、capability 経由に寄せる。

追加 field:

```proto
message RunNormalizationToolRequest {
  string tool_id = 1;
  string tool_version = 2;
  string document_id = 3;
  ToolRunMode mode = 4;
  string job_id = 5;
  string capability_id = 6;
}
```

運用ルール:

- `TOOL_RUN_MODE_DRY_RUN` は `JOB_OPERATION_RUN_NORMALIZATION_TOOL_DRY_RUN` が必要
- `TOOL_RUN_MODE_APPLY` は `JOB_OPERATION_RUN_NORMALIZATION_TOOL_APPLY` と approval が必要

---

## 4. DB schema 変更案

最小差分で入れるなら、既存テーブルの全面作り直しは不要である。  
以下の追加で十分始められる。

### 4.1 `document_processing_jobs` 拡張

追加カラム:

```sql
ALTER TABLE document_processing_jobs
  ADD COLUMN requested_by TEXT NOT NULL DEFAULT '',
  ADD COLUMN capability_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN execution_plan_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN plan_status TEXT NOT NULL DEFAULT '',
  ADD COLUMN evaluation_status TEXT NOT NULL DEFAULT '',
  ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN budget_json TEXT NOT NULL DEFAULT '{}';
```

理由:

- 既存 job をそのまま root aggregate にできる
- capability / plan / eval の現在値を job 行で即取得できる

### 4.2 `job_capabilities`

```sql
CREATE TABLE IF NOT EXISTS job_capabilities (
  capability_id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  graph_id TEXT NOT NULL REFERENCES graphs(graph_id) ON DELETE CASCADE,
  allowed_document_ids_json TEXT NOT NULL DEFAULT '[]',
  allowed_node_ids_json TEXT NOT NULL DEFAULT '[]',
  allowed_operations_json TEXT NOT NULL DEFAULT '[]',
  max_llm_calls INTEGER NOT NULL DEFAULT 0,
  max_tool_runs INTEGER NOT NULL DEFAULT 0,
  max_node_creations INTEGER NOT NULL DEFAULT 0,
  max_edge_mutations INTEGER NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);
```

初期段階では JSON で十分。  
先に API 境界と enforcement を固め、将来必要なら正規化する。

### 4.3 `job_execution_plans`

```sql
CREATE TABLE IF NOT EXISTS job_execution_plans (
  plan_id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  plan_json TEXT NOT NULL,
  created_by TEXT NOT NULL DEFAULT 'planner',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);
```

`plan_json` には `steps`, `risk_tier`, `rollback_strategy` をそのまま入れる。

### 4.4 `job_approval_requests`

```sql
CREATE TABLE IF NOT EXISTS job_approval_requests (
  approval_id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  plan_id TEXT NOT NULL REFERENCES job_execution_plans(plan_id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  requested_operations_json TEXT NOT NULL DEFAULT '[]',
  reason TEXT NOT NULL DEFAULT '',
  risk_tier TEXT NOT NULL DEFAULT '',
  requested_by TEXT NOT NULL DEFAULT 'governor',
  reviewed_by TEXT NOT NULL DEFAULT '',
  requested_at TIMESTAMPTZ NOT NULL,
  reviewed_at TIMESTAMPTZ
);
```

### 4.5 `job_mutation_logs`

worker の動作を説明可能にするため、node / edge mutation をイベントとして残す。

```sql
CREATE TABLE IF NOT EXISTS job_mutation_logs (
  mutation_id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL REFERENCES document_processing_jobs(job_id) ON DELETE CASCADE,
  plan_id TEXT NOT NULL DEFAULT '',
  capability_id TEXT NOT NULL DEFAULT '',
  graph_id TEXT NOT NULL REFERENCES graphs(graph_id) ON DELETE CASCADE,
  target_type TEXT NOT NULL, -- node | edge | tool_run
  target_id TEXT NOT NULL,
  mutation_type TEXT NOT NULL, -- append | revise | relink | remove
  risk_tier TEXT NOT NULL DEFAULT '',
  before_json TEXT NOT NULL DEFAULT '{}',
  after_json TEXT NOT NULL DEFAULT '{}',
  provenance_json TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL
);
```

このテーブルは将来の rollback と監査ログの基盤になる。

### 4.6 `nodes` 拡張

worker に触らせる field と触らせない field を分けるため、最低限以下を足す。

```sql
ALTER TABLE nodes
  ADD COLUMN governance_state TEXT NOT NULL DEFAULT 'system_generated',
  ADD COLUMN locked_by TEXT NOT NULL DEFAULT '',
  ADD COLUMN locked_at TIMESTAMPTZ,
  ADD COLUMN last_mutation_job_id TEXT NOT NULL DEFAULT '';
```

推奨ルール:

- `system_generated`: worker 更新可
- `pending_review`: worker は append 相当の提案のみ可
- `human_curated`: worker は直接上書き不可
- `locked`: read-only

---

## 5. API 境界

### 5.1 Public API

フロントや通常クライアントが叩く API。

対象:

- `GetJobStatus`
- `GetJobExecutionPlan`
- `ListJobApprovalRequests`
- `ApproveJobRequest`
- `RejectJobRequest`

ルール:

- workspace access が前提
- approval 系は editor / owner 以上
- dev 限定にするなら authz で切る

### 5.2 Internal Worker API

API service から worker service を叩く内部境界。

対象:

- `GenerateExecutionPlan`
- `ExecuteApprovedPlan`
- `EvaluateJobArtifact`

ルール:

- 必ず `job_id` と `capability_id` を対で渡す
- worker は DB から capability をロードし、毎回照合する
- caller が内部だからといって capability check を省略しない

### 5.3 Repository 境界

現状の repository interface には governance 情報が無いので、以下の追加が必要。

```go
type JobRepository interface {
    GetCapability(jobID string) (*domain.JobCapability, error)
    SaveExecutionPlan(plan *domain.JobExecutionPlan) error
    GetExecutionPlan(planID string) (*domain.JobExecutionPlan, error)
    CreateApprovalRequest(req *domain.JobApprovalRequest) error
    ListApprovalRequests(jobID string) ([]*domain.JobApprovalRequest, error)
    LogMutation(entry *domain.JobMutationLog) error
}

type NodeRepository interface {
    UpdateNodeWithCapability(nodeID string, patch domain.NodePatch, capability *domain.JobCapability) error
    CreateEdgeWithCapability(graphID, sourceNodeID, targetNodeID, edgeType string, capability *domain.JobCapability) (*domain.Edge, error)
    LockNode(nodeID, actor string) error
}
```

重要なのは、`service でチェックして repository は素通し` にしないこと。  
最終的な enforcement は repository 層でも再確認したほうがよい。

---

## 6. Node mutation の実装ルール

### `append`

- capability に `CREATE_NODE` または `CREATE_EDGE` が必要
- `governance_state` は `system_generated` で作成
- `job_mutation_logs` に `before_json = {}` で記録

### `revise`

- capability に `UPDATE_NODE` が必要
- `human_curated` と `locked` は拒否
- 初期は `description`, `summary_html` のみ対象

### `relink`

- `CREATE_EDGE` と `DELETE_EDGE` の両方を要求
- 変更件数が閾値超過なら approval 必須
- 閾値は初期値 `10` 件程度でよい

### `remove`

- 初期は物理削除禁止
- 代替として tombstone または `pending_review` に落とす

---

## 7. Governor の判定ルール

governor は別サービスにしなくてもよいが、判定規則は固定する。

自動拒否:

- capability に無い operation
- `locked` node への mutation
- `human_curated` node の overwrite
- budget 超過
- `tier_3` 操作の無承認実行

approval 要求化:

- `TOOL_RUN_MODE_APPLY`
- relink の大量実行
- `allowed_node_ids` 外への拡張
- 複数 document をまたぐ再編成

自動許可:

- `tier_0`
- scope 内の `tier_1`

---

## 8. 実装順

実装の順番は以下がよい。

1. schema 追加
2. domain / repository に capability 系 type を追加
3. `JobService` の read API を追加
4. worker 側に `GenerateExecutionPlan` と `EvaluateJobArtifact` を追加
5. node mutation を capability チェック付き helper に集約
6. `RunNormalizationTool` を capability 対応にする

最初から全部自動承認にしない。  
まず `append` と軽微な `revise` だけを worker に許し、`relink` と tool apply は approval 必須で始めるのが安全である。

---

## 9. この仕様で得られるもの

この設計にすると、worker は `成果物のためにかなり広く動ける`。  
一方で、以下は固定できる。

- どの graph / node / document に触れたか
- どの権限で動いたか
- なぜその変更が行われたか
- どの変更に承認が入ったか

つまり、`goal-driven` と `監査可能性` を両立しやすい。
