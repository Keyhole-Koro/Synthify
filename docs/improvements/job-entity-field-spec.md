# Job entity field spec

## ステータス確認 (2026-05-07)

現状の実装とこの指針の乖離状況：
- **DB/Domain 不一致**: `started_at`, `completed_at` は Proto には追加されたが、DB Schema と Domain 型には未実装。
- **機能の空振り**: `current_stage` カラムは DB に存在するが、Worker からの更新処理が未実装のため、UI に進捗が反映されない。
- **Enum 未更新**: `JobLifecycleState` に `waiting_approval` は追加されていない。
- **残骸**: `budget_json` は依然として DB/Domain に残っている。

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

### `job_id`, `workspace_id`, `document_id`, `job_type`, `status`, `current_stage`, `params_json`, `requested_by`, `error_message`, `retry_count`, `created_at`, `started_at`, `completed_at`, `updated_at`

(詳細は各論参照)

## 現状からの改善順

1. [未着手] `started_at` / `completed_at` を schema, domain に追加する。（Proto には追加済みだが不整合あり）
2. [一部完了] proto `Job` に `workspace_id` 等を追加する。（`started_at` / `completed_at` 等は追加済み。`plan_status` / `evaluation_status` は未追加）
3. [未着手] `waiting_approval` / `cancelled` を lifecycle に追加する。
4. [未着手] `budget_json` を削除し、正を `job_capabilities` に寄せる。
5. [未着手] Worker が `current_stage` を更新するようにし、snapshot/checkpoint を実装する。
6. [未着手] Document status と Job status の責務を完全に分離する。
