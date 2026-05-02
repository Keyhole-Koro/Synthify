# Job entity field spec

## 背景

`document_processing_jobs` は、Document を Tree に反映する非同期処理の実行単位を表す。

現状は 1 行の job に以下の関心が集まっている。

- 実行ライフサイクル
- 対象 document / workspace
- worker の現在位置
- 実行計画と承認状態
- capability / budget
- 評価結果
- エラー表示

このドキュメントでは、理想としてどのフィールドが何の処理に必要で、どの判断・副作用に影響するべきかを定義する。

## 基本方針

Job は「処理の実行インスタンス」であり、Document や Tree の実体ではない。

Job 本体に置くべきものは、実行単位として常に必要な identity、対象、lifecycle、進捗、結果の要約に限る。実行計画、承認、capability、mutation log、snapshot は別テーブルまたは別オブジェクトを正とし、job には参照 ID と集約状態だけを置く。

## 関係

```text
Workspace
  ├─ Document
  │    └─ DocumentProcessingJob
  │         ├─ JobCapability
  │         ├─ JobExecutionPlan
  │         ├─ JobApprovalRequest
  │         ├─ JobMutationLog
  │         └─ JobSnapshot / Checkpoint
  └─ TreeItems
```

Job は必ず `workspace_id` と `document_id` の両方を持つ。

`document_id` は入力ソースを決める。`workspace_id` は権限、Tree 更新範囲、検索範囲、capability の境界を決める。`tree_id` を workspace の代わりに使わない。

## Job 本体に持つフィールド

### `job_id`

Job の一意な ID。

必要な処理:

- API からの状態取得
- worker への dispatch
- approval、capability、plan、mutation log、snapshot の join key
- Firestore / realtime status の document key

影響:

- 再実行や再開でも同じ job を継続するか、新しい job を作るかの境界になる。
- `job_id` が変わると capability、budget、mutation log、snapshot も別物になる。

### `workspace_id`

Job が操作してよい workspace。

必要な処理:

- API authorization
- semantic search の workspace 範囲指定
- `tree_items` の read / write 範囲指定
- `job_capabilities.workspace_id` との整合性チェック
- mutation log の partition

影響:

- 間違うと別 workspace の Tree を読んだり更新したりする。
- `CreateProcessingJob` には `tree_id` ではなく、必ず実際の `workspace_id` を渡す。

### `document_id`

Job の主入力 document。

必要な処理:

- GCS object / file metadata の取得
- document chunk の保存・再利用
- source evidence の `item_sources.document_id`
- force reprocess / resume 対象の判定
- capability の `allowed_document_ids`

影響:

- 同じ document に複数 job が存在しうる。
- 最新 job の表示は `document_id` で引けるが、worker 実行は常に `job_id` で扱う。

### `job_type`

Job の意図。

想定値:

- `process_document`: 初回処理
- `reprocess_document`: 既存 document の再処理

必要な処理:

- planning policy の選択
- force reprocess 時の既存 chunk / item / source の扱い
- UI 表示
- capability / plan の初期値決定

影響:

- `process_document` は既存成果物がある場合に安全側で止める選択ができる。
- `reprocess_document` は既存 item との merge、source 更新、snapshot 再利用の方針が変わる。

### `status`

Job の実行ライフサイクル。worker と API が見る最上位状態。

理想の値:

- `queued`: 作成済み、worker 未開始
- `running`: worker 実行中
- `waiting_approval`: 人間の承認待ち。worker は進めない
- `succeeded`: mutation と評価が完了
- `failed`: 回復不能な失敗
- `cancelled`: ユーザーまたはシステムが停止

必要な処理:

- worker が pick できる job の判定
- ResumeProcessing で再開できるかの判定
- UI の大枠表示
- notifier の状態配信

影響:

- `queued` -> `running` は worker が lease を取った時点で行う。
- `running` -> `waiting_approval` は plan が承認を要求した時点で行う。
- `running` -> `succeeded` は mutation、snapshot finalization、evaluation が通った後に行う。
- `running` -> `failed` は retry 不能、または retry 上限到達時に行う。

`status` に細かい stage 名を入れない。細かい進捗は `current_stage` に置く。

### `current_stage`

Worker の現在位置。

例:

- `fetch_source`
- `text_extraction`
- `semantic_chunking`
- `planning`
- `waiting_approval`
- `synthesis`
- `merge`
- `persistence`
- `evaluation`

必要な処理:

- UI の途中経過表示
- snapshot/checkpoint の再開位置
- timeout / stuck job の診断
- worker log と job state の突合

影響:

- `current_stage` は実行制御の補助情報であり、最終的な成功・失敗判定には使わない。
- stage 完了時に snapshot を保存し、次の stage に進む前に更新する。
- `succeeded` / `failed` / `cancelled` では空にするか、最後の stage を保持するかを仕様で固定する。UI 表示を考えると、理想は `last_stage` と `current_stage` を分けること。

### `params_json`

Job 作成時の実行パラメータ。

入れるべき内容:

- `force_reprocess`
- chunking / extraction のモード
- planner options
- requested scope
- snapshot resume policy

必要な処理:

- worker が実行方針を決める
- 同じ document の job 差分を説明する
- retry / resume 時に同じ条件で再実行する

影響:

- 途中で変更しない。Job 作成後は immutable に近い扱いにする。
- 動的に変わる情報は `params_json` ではなく snapshot や plan に置く。

### `requested_by`

Job を開始した user / system。

必要な処理:

- audit
- approval request の初期 requester
- UI 表示
- rate limit / quota の帰属

影響:

- `system` と user id を混ぜない。system job は明示的に `system` または service account id として扱う。

### `error_message`

ユーザーまたは運用者に見せる失敗要約。

必要な処理:

- failed job の UI 表示
- retry 可否の判断材料
- monitoring alert

影響:

- 詳細 log を詰め込まない。詳細は worker log / snapshot / error table に置く。
- retry で `running` に戻す時はクリアする。

### `retry_count`

同一 job の retry 回数。

必要な処理:

- retry 上限判定
- backoff 計算
- stuck job recovery

影響:

- retry のたびに increment する。
- 新しい job を作る reprocess では 0 から始める。
- stage retry と job retry を分けたい場合は snapshot 側に stage attempt を持たせる。

### `created_at`

Job 作成時刻。

必要な処理:

- document の最新 job 判定
- UI ソート
- stale job 判定

影響:

- immutable。

### `started_at`

Worker が実行を開始した時刻。

必要な処理:

- queue latency の計測
- running timeout の判定
- UI 表示

影響:

- `queued` -> `running` の時点で一度だけ設定する。
- 現状の schema にはないため、`updated_at` で代用しない方がよい。

### `completed_at`

Job が terminal state に入った時刻。

対象 terminal state:

- `succeeded`
- `failed`
- `cancelled`

必要な処理:

- duration 計測
- UI 表示
- cleanup / retention policy

影響:

- `updated_at` とは意味が違う。途中 stage 更新でも `updated_at` は変わるため、`completed_at` に流用しない。

### `updated_at`

Job row の最終更新時刻。

必要な処理:

- polling / realtime sync の差分検知
- stuck job の検出
- optimistic update

影響:

- stage 更新、plan state 更新、evaluation 更新でも変わる。
- 完了時刻として使わない。

## Job 本体には参照だけ置くフィールド

### `capability_id`

正は `job_capabilities`。

Job 本体の `capability_id` は、現在有効な capability を引くための denormalized pointer。

必要な処理:

- worker tool の権限チェック
- LLM / tool / item creation budget の enforcement
- allowed document / item / operation の検証

影響:

- capability がない job は worker が mutation してはいけない。
- capability の中身を job row の `budget_json` と二重管理しない。理想は `job_capabilities` を正にして、`budget_json` は削除または snapshot 表示用に限定する。

### `execution_plan_id`

正は `job_execution_plans`。

必要な処理:

- 承認対象 plan の特定
- worker が実行してよい plan の特定
- mutation log の plan provenance

影響:

- plan が差し替わる場合、古い plan と approval の関係を失わない。
- job row には current plan id だけを置き、履歴は `job_execution_plans` に残す。

### `plan_status`

正は `job_execution_plans.status` と `job_approval_requests.status`。

Job 本体の `plan_status` は UI / worker dispatch 用の集約状態。

理想の値:

- `none`: plan 未作成
- `draft`: plan 作成中
- `pending_approval`: 承認待ち
- `approved`: 実行可能
- `executing`: plan 実行中
- `completed`: plan 実行完了
- `rejected`: 承認却下

必要な処理:

- worker が execution に進んでよいか
- UI が承認ボタンを出すか
- ResumeProcessing が approval 後に続行できるか

影響:

- `status=waiting_approval` と `plan_status=pending_approval` は同時に成立する。
- `status=running` でも `plan_status=pending_approval` なら mutation はしてはいけない。

### `evaluation_status`

正は evaluation result table があるのが理想。現状は job row に集約されている。

理想の値:

- `pending`
- `running`
- `passed`
- `failed`
- `skipped`

必要な処理:

- `succeeded` にしてよいかの gate
- UI の品質表示
- retry / manual review の判断

影響:

- mutation が成功しても evaluation が failed なら job は `failed` または `succeeded_with_warnings` 相当にする判断が必要。
- 現在の enum には `succeeded_with_warnings` がないため、理想では `status` と `evaluation_status` を分けて表示する。

### `budget_json`

理想では削除候補。

正は `job_capabilities` と usage counters。

残す場合の用途:

- job 作成時の budget snapshot
- UI 表示用の冗長キャッシュ

影響:

- enforcement に使うと `job_capabilities` と不整合が起きる。
- 実使用量は `budget_json` ではなく usage counters / snapshot に置く。

## 別オブジェクトに持つべきもの

### JobCapability

Job の実行権限と上限。

持つべき内容:

- `allowed_document_ids`
- `allowed_item_ids`
- `allowed_operations`
- `max_llm_calls`
- `max_tool_runs`
- `max_item_creations`
- `expires_at`

処理への影響:

- tool 実行前に operation を検証する。
- LLM 呼び出し前に `JOB_OPERATION_INVOKE_LLM` と `max_llm_calls` を検証する。
- item 永続化前に `max_item_creations` を検証する。

### JobExecutionPlan

Worker が実行する計画。

持つべき内容:

- plan steps
- risk tier
- required operations
- target document / item
- generated signals
- approval requirement

処理への影響:

- approval が必要かを決める。
- worker がどの stage / tool を呼ぶかを決める。
- mutation log の provenance になる。

### JobApprovalRequest

人間の承認状態。

処理への影響:

- pending の間は destructive mutation を止める。
- approved 後に `plan_status=approved` とし、worker resume を可能にする。
- rejected 後は job を `failed` または `cancelled` に寄せる。

### JobMutationLog

Tree や item に加えた変更の監査ログ。

処理への影響:

- evaluation の入力になる。
- rollback / explain の根拠になる。
- item の `last_mutation_job_id` と対応する。

### JobSnapshot / Checkpoint

Worker の復帰用 state。

持つべき内容:

- completed stages
- stage payload refs
- usage counters
- generated intermediate outputs
- last successful tool call
- resumable / non-resumable reason

処理への影響:

- retry 時に完了済み stage を skip する。
- 同じ LLM 呼び出しや同じ persistence を避ける。
- `current_stage` より細かい復帰判断を行う。

## 状態遷移

```text
queued
  -> running
  -> waiting_approval
  -> running
  -> succeeded

queued
  -> running
  -> failed

queued
  -> cancelled

running
  -> cancelled
```

理想では、承認待ちを `status=running` の一種にしない。worker が進めない状態なので `waiting_approval` を lifecycle に入れる方が分かりやすい。

## Document status との関係

Document と Job の status は分ける。

Document status は「その document の処理結果として今ユーザーに何を見せるか」を表す。

Job status は「特定の実行インスタンスがどうなっているか」を表す。

例:

- document は `completed`
- 最新 reprocess job は `failed`

この状態はありうる。過去に成功した Tree を表示しながら、最新再処理の失敗を通知するため。

## API に返す Job

API の `Job` message は、最低限以下を返すべき。

```proto
message Job {
  string job_id = 1;
  string workspace_id = 2;
  string document_id = 3;
  JobType type = 4;
  JobLifecycleState status = 5;
  string current_stage = 6;
  string plan_status = 7;
  string evaluation_status = 8;
  string created_at = 9;
  string started_at = 10;
  string completed_at = 11;
  string updated_at = 12;
  string error_message = 13;
}
```

`started_at` と `completed_at` を `updated_at` で代用しない。

## 現状からの改善順

1. `CreateProcessingJob` に `workspace_id` を正しく渡す。
2. `started_at` / `completed_at` を schema、domain、proto に追加する。
3. proto `Job` に `workspace_id`、`current_stage`、`plan_status`、`evaluation_status`、`updated_at` を追加する。
4. `waiting_approval` / `cancelled` を lifecycle に追加するか、既存 enum のままなら代替ルールを明文化する。
5. `budget_json` を enforcement に使わないことを固定し、正を `job_capabilities` に寄せる。
6. snapshot/checkpoint を追加し、`current_stage` は表示用、snapshot は復帰用として分ける。
7. Document status と Job status の責務を分離する。

