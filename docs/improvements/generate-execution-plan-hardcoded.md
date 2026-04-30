# GenerateExecutionPlan がハードコードされたステップ列を返すだけ

## 場所
`worker/pkg/worker/worker.go` — `Planner.GenerateExecutionPlan`

## 問題
プランのステップ列が毎回固定の5ステップにハードコードされており、LLM も signals も実際の判断に使われていない。

```go
"steps": []map[string]any{
    {"name": "text_extraction",      "operation": "JOB_OPERATION_READ_DOCUMENT", "risk_tier": "tier_0"},
    {"name": "semantic_chunking",    "operation": "JOB_OPERATION_INVOKE_LLM",    "risk_tier": "tier_0"},
    {"name": "goal_driven_synthesis","operation": "JOB_OPERATION_CREATE_ITEM",   "risk_tier": "tier_1"},
    {"name": "persistence",          "operation": "JOB_OPERATION_CREATE_ITEM",   "risk_tier": "tier_1"},
    {"name": "evaluation",           "operation": "JOB_OPERATION_EMIT_EVAL",     "risk_tier": "tier_0"},
},
```

`GetJobPlanningSignals` で `same_document_item_count` などを取得しているが、その値を見てステップを変える分岐が存在しない。結果として：

- 再処理時に既存 items の削除ステップが入らない
- ドキュメントの種別・状態に応じた分岐ができない
- `ExecuteApprovedPlan` 側もプランの内容を見ておらず、ADK エージェントに丸投げしている

## 修正方針
signals を実際に参照してステップを動的に決定する。最低限：

- `same_document_item_count > 0` → `delete_existing_items` ステップを先頭に追加（再処理ルール、[tree-lifecycle-multi-document.md](tree-lifecycle-multi-document.md) 参照）
- 将来的には LLM にプラン生成を委ねる（ドキュメント種別・サイズ・目的に応じたプランニング）

`ExecuteApprovedPlan` 側もプラン JSON のステップを読んで実行内容を制御する必要がある。
