# Improvements

既知の問題点・改善候補の一覧。優先度順に並べている。

## P1 — 設計上の問題

- [capability-limits-not-enforced.md](capability-limits-not-enforced.md) — JobCapability のLLM呼び出し上限が実際には強制されていない
- [generate-execution-plan-hardcoded.md](generate-execution-plan-hardcoded.md) — GenerateExecutionPlan がハードコードされたステップ列を返すだけで signals を使っていない
- [force-reprocess-ignored.md](force-reprocess-ignored.md) — forceReprocess パラメータが無視されており再処理が機能しない
- [agent-error-silenced.md](agent-error-silenced.md) — ADK エージェント実行エラーが握りつぶされジョブが成功扱いになる可能性がある
- [mock-workspace-access-always-true.md](mock-workspace-access-always-true.md) — mock の IsWorkspaceAccessible が常に true でアクセス制御テストが壊れている

## P2 — スタブ・簡易実装

- [worker-tools-stub.md](worker-tools-stub.md) — synthesis/merging/briefing/critique ツールが簡易実装のまま（詳細設計: [process-tools-llm-implementation.md](process-tools-llm-implementation.md)）
- [resume-processing-stub.md](resume-processing-stub.md) — ResumeProcessing がダミー job_id を返すだけで実際の再開ロジックがない

## P3 — 仕様ドラフト（実装前に設計が必要）

- [tree-lifecycle-multi-document.md](tree-lifecycle-multi-document.md) — 複数ドキュメント処理時の tree 統合・更新ライフサイクル（Phase 1〜3）

## Future Improvements（別ファイル）

- [../../docs/llm-worker-tools.md](../llm-worker-tools.md) — semantic_search の two-stage re-rank、PDF/画像対応

## アーキテクチャドキュメント

- [../llm-worker-architecture.md](../llm-worker-architecture.md) — LLM Worker の設計思想・ツール層構造・責任分界
- [../llm-worker-simulation.md](../llm-worker-simulation.md) — API設計書を例にしたターンごとの処理シミュレーション
