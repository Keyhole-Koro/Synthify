# Current State: Data, Lifecycle, and Messaging

このドキュメントは Synthify の現状を「ありのまま」記述したスナップショットである。**「あるべき形」ではなく「今そうなっている形」を正確に残す**ことだけを目的とする。設計の見直し作業の出発点として、何が歪んでいるかを論じる前に共通の認識を作るために書く。

設計案や改善方針は [target-design.md](target-design.md) と [migration-plan.md](migration-plan.md) (執筆予定) に分けて記載する。

---

## 1. 永続化されているデータ

### 1.1 全体像

23 テーブルが 6 ドメインに分かれている。

| ドメイン | テーブル数 | 代表テーブル |
|---|---|---|
| Account / User | 3 | `accounts`, `account_users`, `users` |
| Workspace | 1 | `workspaces` |
| Document / Upload | 4 | `documents`, `document_files`, `document_chunks`, `upload_reservations` |
| Tree / Item | 3 | `tree_items`, `item_sources`, `item_aliases` |
| Processing Job | 6 | `document_processing_jobs`, `job_capabilities`, `job_execution_plans`, `job_mutation_logs`, `job_logs`, `job_approval_requests`, `job_stage_checkpoints` |
| Billing / Usage | 6 | `accounts` (兼用), `usage_events`, `account_usage_daily`, `model_pricing`, `account_credits`, `invoices`, `payment_methods`, `billing_events` |
| Dynamic Tools | 1 | `dynamic_tools` |

### 1.2 Account / User

**accounts** (PK `account_id`, no FK)
- ビリングの主体。Stripe サブスクリプション、ストレージ容量、月次予算上限を持つ。
- 注目カラム: `storage_quota_bytes`, `storage_used_bytes`, `budget_limit_minor`, `current_period_usage_minor`, `budget_exceeded`, `cancel_at_period_end`, `billing_status`

**account_users** (PK `(account_id, user_id)`)
- `account_id` → `accounts` ON CASCADE。`user_id` は文字列で **users への FK は無い**。`role` でアクセス階層。

**users** (PK `user_id`, no FK)
- マイグレーション 0012 で後付け追加された独立した user テーブル。`email`, `display_name`, `last_login_at`。`accounts` と直接の関係なし。

### 1.3 Workspace

**workspaces** (PK `workspace_id`, FK `account_id` → `accounts` ON CASCADE)
- マイグレーション 0015 で `deleted_at` (nullable) が追加されソフト削除可。ソフト削除はワークスペースだけに導入されており他テーブルには波及していない。

### 1.4 Document / Upload

**documents** (PK `document_id`, FK `workspace_id` → `workspaces` ON CASCADE)
- 内容ではなくメタデータのみ。`uploaded_by`, `file_size`, `mime_type`。`(workspace_id, created_at DESC)` に index。

**document_files** (PK `file_id`, FK `document_id` → `documents` ON CASCADE)
- 1 ドキュメントが複数ファイルを持つケースを許容する設計。`(document_id, path)` ユニーク。

**document_chunks** (PK `chunk_id`, FK `document_id`, `file_id` nullable cascade)
- pgvector の `VECTOR(768)` カラム `embedding` を持つ。`heading`, `source_page` 等。`chunk_id` のデフォルトが空文字列で、孤児 chunk を許容する作りになっている。

**upload_reservations** (PK `reservation_id`, FK `account_id`/`workspace_id`/`document_id` cascade)
- 署名 URL 発行と Postgres 状態のブリッジ。`status` (reserved → confirmed → expired/failed)、`expected_size_bytes` vs `actual_size_bytes`。`document_id` にユニーク制約 (1 ドキュメント 1 予約)。

### 1.5 Tree / Item

**tree_items** (PK `id`, FK `workspace_id` → `workspaces`, `parent_id` → 自己参照 ON DELETE SET NULL)
- ワークスペース知識グラフのノード。`governance_state` (例: `system_generated`, `approved`)、`last_mutation_job_id`、`level`, `description`, `content`, `override_css`, `created_by`。
- `parent_id` の `SET NULL` により、親削除で子が「ルートを失った状態」で残る可能性 (孤児)。
- **"document root" / "workspace root" の区別はバックエンドスキーマには存在しない**。ワークスペースの root は「`parent_id IS NULL` の `tree_items`」として暗黙に定義される。document に紐づく item は `item_sources` 経由で間接的に分かるが、ワークスペース root の直下 child を「document root」とみなすのはフロント側の慣習。

**item_sources** (PK `(item_id, document_id, chunk_id)`, FK 全 cascade)
- tree item と chunk の証拠リンク。`source_text`, `confidence`。

**item_aliases** (PK `(workspace_id, canonical_item_id, alias_item_id)`, FK cascade)
- ステージング: 重複/類似 item を統合する承認待ち。`status` (`pending`, `approved`, `rejected`)。

### 1.6 Processing Job

**document_processing_jobs** (PK `job_id`, FK `document_id`/`workspace_id` cascade)
- 本ドキュメントの主役。ジョブ実行の root エンティティ。`status`, `current_stage`, `job_type`, `params_json`, `budget_json`, `capability_id`, `execution_plan_id`, `plan_status`, `evaluation_status`, `retry_count`, `error_message`。
- 状態は文字列列挙: `queued` / `running` / `succeeded` / `failed`。

**job_capabilities** (PK `capability_id`, FK `job_id`/`workspace_id` cascade)
- ジョブのサンドボックス境界。`allowed_document_ids_json`, `allowed_item_ids_json`, `allowed_operations_json` は JSON 配列で正規化されていない。`max_llm_calls`, `max_tool_runs`, `max_item_creations`, `expires_at`。

**job_execution_plans** (PK `plan_id`, FK `job_id` cascade)
- LLM 計画フェーズの出力。`status` (`draft`, `approved`, `executing`)、`plan_json` にステップ配列、`summary`。

**job_mutation_logs** (PK `mutation_id`, FK `job_id`/`workspace_id` cascade; `plan_id`/`capability_id` は FK 無し)
- ジョブが行った state 変更の監査ログ。`target_type`/`target_id`, `mutation_type`, `before_json`/`after_json`, `provenance_json`, `risk_tier`。

**job_logs** (PK `id`, FK `job_id`/`workspace_id` cascade; `document_id` 任意)
- 構造化イベントログ。`level`, `event`, `message`, `detail_json`。`(job_id, created_at)`, `(workspace_id, created_at)`, `(level, created_at)` に index。
- **stdout に出る slog とは別**。`postgres.NewDBLogger` がジョブ実行中にここに書き込み、Monitor UI が読み出す。

**job_approval_requests** (PK `approval_id`, FK `job_id`/`plan_id` cascade)
- 人手承認フロー。`status` (`pending`, `approved`, `rejected`)、`requested_operations_json`, `reviewed_by`, `reviewed_at`。

**job_stage_checkpoints** (PK `(job_id, stage)`, FK `job_id` cascade)
- 多段ジョブの再開ポイント。`status` (`running`/`succeeded`/`failed`)、`gcs_ref` (チェックポイントの GCS object パス、スキーマ検証なし)。

### 1.7 Billing / Usage

`accounts` 兼用に加えて、`usage_events`、`account_usage_daily` (日次集計, PK `(account_id, usage_date, model)`)、`model_pricing` (PK `model`)、`account_credits` (前払い残高)、`invoices`、`payment_methods`、`billing_events` (Stripe webhook 冪等性、PK `(provider, event_id)`)。

### 1.8 Dynamic Tools

**dynamic_tools** (PK `tool_id`, FK `origin_workspace_id` → `workspaces` cascade; `origin_job_id` は FK 無し)
- LLM 生成ツール (Python / Starlark)。`scope` (workspace/global)、`declared_tier` vs `floor_tier` vs `risk_tier` の三層、`status` (candidate/active/held/rejected/disabled)。プロモーション・ワークフロー。

### 1.9 注意すべき構造的特徴

1. **users への FK が無い箇所が複数**。`account_users.user_id`、`usage_events`、`tree_items.created_by`、`dynamic_tools.origin_job_id` 等が文字列のまま。意図的な疎結合だが orphan を許す。
2. **JSON 配列でリレーションを持つ箇所がある**。`job_capabilities.allowed_*_ids_json` や `item_aliases` 状態。正規化された junction table を使っていない。
3. **soft delete はワークスペースだけ**。他テーブルは hard delete (cascade)。
4. **チェックポイントの GCS ref に integrity check がない**。`gcs_ref` 文字列が壊れていても DB レベルでは検知不可。
5. **billing_events に FK が無い**。webhook が account 同期より先に来ることを許容する設計。
6. **`tree_items.parent_id` の SET NULL** で親消去時に子が孤児になる。明示的な reparent / cascade delete のロジックが要る。

---

## 2. ジョブの状態機械

### 2.1 Postgres の `document_processing_jobs.status`

状態遷移を書き込む関数は以下:

| 関数 | 遷移 | 呼び出し元 |
|---|---|---|
| `MarkProcessingJobRunning(jobID)` | `queued` → `running` | `joblifecycle.Service.MarkRunning` (api と worker の両方にコピーあり) |
| `UpdateProcessingJobStage(jobID, stage)` | `running` のまま `current_stage` 更新 | `pipeline.Runner` 内 |
| `FailProcessingJob(jobID, errorMessage)` | `running` → `failed` | `joblifecycle.Service.TryFail` (api/worker 両方); worker の `failJob` |
| `CompleteProcessingJob(jobID)` | `running` → `succeeded` | `joblifecycle.Service.Complete` (api/worker 両方); `pipeline.Runner` |

**queued の Postgres 書き込みは無い**。ジョブ作成時 (`CreateProcessingJob`) はステータスを直接 `queued` で挿入する。`NotifyQueued` は Firestore にだけ書き込み、Postgres は読み書きしない。

### 2.2 Firestore Mirror

書き込み先: `workspaces/{wsId}/jobs/{jobId}` (1 ドキュメント / ジョブ)

| Notifier メソッド | status | currentStage | progress | message | その他 |
|---|---|---|---|---|---|
| `Queued` | `queued` | "" | 0 | "Queued" | `createdAt` 初期化 |
| `Running` | `running` | "" | 5 | "Processing started" | `startedAt` 設定 |
| `Stage` / `StageProgress` | `running` (不変) | stage 名 | optional | optional | — |
| `Completed` | `succeeded` | "" | 100 | "Completed" | `completedAt`, `expiresAt` (now+7d) |
| `Failed` | `failed` | "" | — | "Failed" | `errorMessage`, `completedAt`, `expiresAt` (now+7d) |

スキーマは [contracts/firestore/job-status.schema.json](../../contracts/firestore/job-status.schema.json) に固定され、Go / TS 型は generator から出力される。書き込みは全て `firestore.MergeAll` で冪等。

### 2.3 Postgres / Firestore の不整合シナリオ

**書き込み順序**: `joblifecycle.Service` は **Postgres を先に**書き、その後 Firestore を更新する。例: `MarkRunning` は `repo.MarkProcessingJobRunning(jobID)` 成功後に `notifier.Running(payload)` を呼ぶ ([apps/api/internal/job/lifecycle/service.go:57-68](../../apps/api/internal/job/lifecycle/service.go#L57-L68))。

Postgres 失敗時は Firestore 呼び出しに進まない。Firestore 失敗時は slog Warn を出して続行する (= **Postgres は更新済みなのに Firestore は古い値のまま**)。

**現れる症状**:
1. Postgres OK / Firestore NG: `processing_jobs.status = running` だが Firestore は `queued` のまま。フロントは「queued がいつまでも続いている」ように見える。
2. Postgres NG: Firestore は更新されない。エラーが呼び出し側に返る。
3. **トランザクション境界は無い**。複数の Service メソッドが順に呼ばれる場合、その間に他のリーダーが中間状態を観測しうる。

**冪等性**:
- Firestore: `MergeAll` で書き直しても問題ない。
- Postgres: 単純な UPDATE で `ON CONFLICT` ハンドリングは無い。ジョブ単位でシングルスレッドが前提。

### 2.4 状態機械のコードの所在

`joblifecycle.Service` は **api と worker の両方に同名の独立コピーがある** ([apps/api/internal/job/lifecycle/service.go](../../apps/api/internal/job/lifecycle/service.go) と [apps/worker/pkg/worker/job/lifecycle/service.go](../../apps/worker/pkg/worker/job/lifecycle/service.go))。コード自体はほぼ同一。

呼び出し関係:
- api: `DocumentService.startProcessingJob` (ジョブ作成) → `lifecycle.NotifyQueued` → dispatcher.GenerateExecutionPlan / dispatcher.ExecuteApprovedPlan で worker を呼ぶ
- worker: `Worker.Process` (= ExecuteApprovedPlan RPC ハンドラ) → `lifecycle.MarkRunning` → orchestrator → `lifecycle.Complete` または `failJob`/`lifecycle.TryFail`
- worker pipeline (`pkg/worker/pipeline/runner.go`): ステージ単位で `UpdateProcessingJobStage`, `CompleteProcessingJob` を呼ぶ別経路がある

つまり **「ジョブの状態遷移を起こす場所が複数のパッケージに散らばっており、唯一の中心を持たない**」。

---

## 3. メッセージング

### 3.1 api ↔ worker (Connect RPC)

定義: [contracts/connectrpc/synthify/worker/v1/](../../contracts/connectrpc/synthify/worker/v1/) 配下 (worker service)。

| RPC | 呼ぶ側 | 用途 |
|---|---|---|
| `GenerateExecutionPlan` | api → worker | ジョブ実行計画の LLM 生成。`processing_jobs.execution_plan_id` を書く |
| `ExecuteApprovedPlan` | api → worker | 承認済みプランの実行。`Worker.Process` を起動 |
| `EvaluateJobArtifact` | api → worker | ジョブ評価 (現状用途限定) |

payload: `ExecutePlanRequest { jobId, jobType, documentId, workspaceId, treeId, fileUri, filename, mimeType }`。完了通知は **同期レスポンス** に乗り、結果情報は `{status: "ok"}` のみ。「何が新規にできたか」は返らない。

### 3.2 frontend ↔ api (Connect RPC)

定義: [contracts/connectrpc/synthify/app/v1/](../../contracts/connectrpc/synthify/app/v1/)。

主要サービス (ジョブ関連):
- `DocumentService`: `CreateDocument`, `ConfirmUpload`, `StartProcessing`, `ResumeProcessing`, `GetDocument`, `ListDocuments`
- `JobService`: `GetJobStatus`, `GetJobExecutionPlan`, `ListJobApprovalRequests`, `RequestJobApproval`, `Approve/RejectJobApproval`, `ListJobLogs`, `SearchJobLogs`, `ListRelatedJobLogs`, `ListAllJobs`, `ListJobMutationLogs`
- `TreeService`: `GetTree(workspaceId)`, `GetSubtree(workspaceId, itemId, maxDepth)`, `FindPaths(...)`

`Job` proto メッセージ (job.proto):

```
Job {
  string job_id;
  string document_id;
  JobType type;
  JobLifecycleState status;
  string created_at;
  string started_at;
  string completed_at;   // 実装は updated_at をマップ
  string error_message;
  string workspace_id;
}
```

**`completed_at` の値が `updated_at` にマップされている** ([apps/api/internal/handler/job.go:336](../../apps/api/internal/handler/job.go#L336))。つまり Postgres の `processing_jobs` には独立した `completed_at` カラムは無く、`updated_at` で代用。

### 3.3 frontend ↔ Firestore (リアルタイムリスナー)

- [apps/web/src/features/jobs/useJobStatus.ts](../../apps/web/src/features/jobs/useJobStatus.ts) — `doc(db, 'workspaces', wsId, 'jobs', jobId)` を購読
- [apps/web/src/features/jobs/useWorkspaceJobStatuses.ts](../../apps/web/src/features/jobs/useWorkspaceJobStatuses.ts) — `collection(...).orderBy('updatedAt', 'desc').limit(6)` を購読

フロントは **ジョブの状態取得には Firestore リスナーを使う**。Connect の `GetJobStatus` は使われていない (job 一覧画面の admin 用途と監査向け)。

### 3.4 worker → Firestore

worker 側で `jobstatus.Notifier` を介してのみ書き込む。直接 Firestore SDK を呼ぶ箇所は他にない。書き込み経路は前述の `lifecycle.Service` 経由のみ。

---

## 4. 語彙のズレ

### 4.1 "tree"

- **Postgres**: `tree_items` テーブルがあるが、`trees` テーブルは無い。ツリーはワークスペースに 1 つ暗黙に存在し、`tree_items.workspace_id` で識別される。
- **proto/Go (`domain.Tree`)**: `Tree { tree_id, workspace_id, ... }` という構造体があり、tree_id は API レベルで露出している。実体は `workspaceId === treeId` のケースが多く、`GetOrCreateTree(workspaceID)` が呼ばれている。
- **frontend**: `getTree(workspaceId)` で取得 (item の配列を返す)。フロントは tree を「ワークスペース内の item 一覧」と同義に扱っている。

→ **tree が独立した一級概念か、ワークスペースの別名か曖昧**。

### 4.2 "workspace root" と "document root"

- **Postgres**: どちらも存在しない概念。`tree_items.parent_id IS NULL` の item が暗黙の「workspace root」だが、明示的なフラグはない。
- **frontend**: [apps/web/src/features/workspaces/useWorkspaceTree.tsx](../../apps/web/src/features/workspaces/useWorkspaceTree.tsx) で `findRootItemId(items)` でルートを探し、その `childIds` を `documentRootIds` と呼んで使う。
- **「document root」はフロント側の慣習表現**で、バックエンドのスキーマ・API・コードのどこにも明示されていない。

→ **同じ概念がフロントとバックエンドで違う名前で呼ばれており、バックエンドでは命名すらされていない**。

### 4.3 "job lifecycle state" と "processing job status"

- proto enum: `JobLifecycleState_JOB_LIFECYCLE_STATE_{QUEUED, RUNNING, SUCCEEDED, FAILED}`
- Postgres カラム: `document_processing_jobs.status` (文字列)
- Firestore: `status` (文字列)、`FirestoreJobStatusStateQueued` 等の Go constant あり

各レイヤで列挙の型/名前が違う。実値は同じだが、変換ロジックがハンドラに散らばる。

### 4.4 "completed_at"

- Postgres には `completed_at` カラムが無い。`updated_at` で代用。
- proto `Job.completed_at` は handler で `updated_at` をマップしている。
- Firestore は独立した `completedAt` フィールドを持つ。

→ **同じ意味のフィールドが 3 レイヤで実装方式が違う**。

---

## 5. 観測まとめ

このスナップショットから読み取れる現状の構造的な特徴:

1. **ジョブの状態管理が「中心を持たない」**。`joblifecycle.Service` のコピーが api と worker に並存し、状態遷移の起点が複数の Service / pipeline に散らばっている。
2. **Postgres と Firestore の同期はベストエフォート**。ドリフトは設計上発生しうる。ただし「Postgres が真、Firestore は表示用 mirror」という非対称性は実装上一貫している (Firestore 失敗で Postgres は元に戻さない)。
3. **「document root」がフロント固有の概念**。バックエンドはこれを語彙として持っておらず、フロントが workspace root の子で推測している。
4. **`processing_jobs.completed_at` 不在**。`updated_at` でラベル違いの代用が発生している。
5. **ジョブ完了通知に「成果物の参照」が無い**。完了 RPC レスポンスは `{status: "ok"}` のみで、何が新規に作られたかは別途 tree 全取得で発見するしかない。
6. **api と worker の repository / lifecycle がコピペ**。コードの整合性は手動メンテに依存している。
7. **soft delete はワークスペースだけ**。ドキュメント・アイテム・ジョブの cascade hard delete によって履歴が完全に消える可能性。
8. **JSON カラムでリレーション表現が複数箇所**。`allowed_*_ids_json` 等。

これらは [target-design.md](target-design.md) (執筆予定) で「どうあるべきか」を議論する対象になる。

---

## 6. 関連ファイル

主要なコードの所在:

| 関心事 | 場所 |
|---|---|
| Postgres スキーマ | [db/migrations/](../../db/migrations/) |
| sqlc 生成 | [apps/api/internal/repository/postgres/sqlcgen/](../../apps/api/internal/repository/postgres/sqlcgen/) |
| Repository 実装 (api) | [apps/api/internal/repository/postgres/](../../apps/api/internal/repository/postgres/) |
| Repository 実装 (worker) | [apps/worker/pkg/worker/repository/postgres/](../../apps/worker/pkg/worker/repository/postgres/) |
| Service 層 (api) | [apps/api/internal/service/](../../apps/api/internal/service/) |
| Worker pipeline | [apps/worker/pkg/worker/](../../apps/worker/pkg/worker/) |
| Job lifecycle (api コピー) | [apps/api/internal/job/lifecycle/service.go](../../apps/api/internal/job/lifecycle/service.go) |
| Job lifecycle (worker コピー) | [apps/worker/pkg/worker/job/lifecycle/service.go](../../apps/worker/pkg/worker/job/lifecycle/service.go) |
| Firestore notifier | [internal/platform/job/status/notifier.go](../../internal/platform/job/status/notifier.go) |
| Firestore schema | [contracts/firestore/job-status.schema.json](../../contracts/firestore/job-status.schema.json) |
| Connect proto (app) | [contracts/connectrpc/synthify/app/v1/](../../contracts/connectrpc/synthify/app/v1/) |
| Connect proto (worker) | [contracts/connectrpc/synthify/worker/v1/](../../contracts/connectrpc/synthify/worker/v1/) |
| Frontend ジョブ購読 | [apps/web/src/features/jobs/](../../apps/web/src/features/jobs/) |
| Frontend ツリー表示 | [apps/web/src/features/workspaces/useWorkspaceTree.tsx](../../apps/web/src/features/workspaces/useWorkspaceTree.tsx) |
