# Improvements

既知の問題点・改善候補の一覧。優先度順に並べている。

## アーキテクチャドキュメント (確定済み)

- [../architecture/gcs-fuse-ingestion-spec.md](../architecture/gcs-fuse-ingestion-spec.md) — **Done**: GCS FUSE 活用、ディレクトリ統一戦略、およびインジェクション・フローの正式仕様。
- [../architecture/job-checkpoint-spec.md](../architecture/job-checkpoint-spec.md) — **Done**: ジョブの状態復帰 (Snapshot/Checkpoint) に関する正式仕様。
- [../llm-worker-architecture.md](../llm-worker-architecture.md) — LLM Worker の設計思想・ツール層構造・責任分界
- [../llm-worker-simulation.md](../llm-worker-simulation.md) — API設計書を例にしたターンごとの処理シミュレーション

## P1 — 設計上の問題

- [dependency-architecture-ideal.md](dependency-architecture-ideal.md) — `root` `api` `worker` `shared` `web` `log-viewer` の理想依存構成と段階的移行方針
- [api-refactor-cleanup.md](api-refactor-cleanup.md) — API 層の仕様変更残骸、mapper 不整合、重複 dispatch、no-op RPC の整理
- [refactor-backlog-008-010.md](refactor-backlog-008-010.md) — `sqlc` 移行残件、job/document 状態遷移集約、テスト fixture 共通化の具体作業
- [job-entity-field-spec.md](job-entity-field-spec.md) — Job エンティティの理想フィールド、状態遷移、処理への影響
- [capability-limits-not-enforced.md](capability-limits-not-enforced.md) — JobCapability のLLM呼び出し上限が実際には強制されていない
- [generate-execution-plan-hardcoded.md](generate-execution-plan-hardcoded.md) — GenerateExecutionPlan がハードコードされたステップ列を返すだけで signals を使っていない
- [force-reprocess-ignored.md](force-reprocess-ignored.md) — forceReprocess パラメータが無視されており再処理が機能しない
- [agent-error-silenced.md](agent-error-silenced.md) — ADK エージェント実行エラーが握りつぶされジョブが成功扱いになる可能性がある
- [mock-workspace-access-always-true.md](mock-workspace-access-always-true.md) — mock の IsWorkspaceAccessible が常に true でアクセス制御テストが壊れている

## P2 — スタブ・簡易実装

- [worker-tools-stub.md](worker-tools-stub.md) — synthesis/merging/briefing/critique ツールが簡易実装のまま（詳細設計: [process-tools-llm-implementation.md](process-tools-llm-implementation.md)）
- [resume-processing-stub.md](resume-processing-stub.md) — ResumeProcessing がダミー job_id を返すだけで実際の再開ロジックがない

## P3 — 仕様ドラフト（実装前に設計が必要）

- [usage-based-billing.md](usage-based-billing.md) — LLM 従量課金への移行設計（usage metering、予算アラート、Worker 緊急停止、Stripe Meters 連携）
- [tree-lifecycle-multi-document.md](tree-lifecycle-multi-document.md) — 複数ドキュメント処理時の tree 統合・更新ライフサイクル（Phase 1〜3）
- [router-job-splitting.md](router-job-splitting.md) — 巨大ドキュメントをジョブ分割して Router プロキシで処理する設計（未決定事項あり）
- [workspace-paper-compact-ui.md](workspace-paper-compact-ui.md) — tree 生成後は workspace paper を compact handle にし、document roots を直接 child papers として見せる UI 仕様
- [paper-in-paper-sibling-share.md](paper-in-paper-sibling-share.md) — sibling 内 room 配分、focus のシーソー挙動、初期 open state / persisted state 優先の設計メモ
- [paper-in-paper-importance-direction.md](paper-in-paper-importance-direction.md) — subtree 加算型 importance をやめ、current attention に room を追従させる設計比較と推奨方針
- [log-viewer-bi-dashboards.md](log-viewer-bi-dashboards.md) — log-viewer を BI 化する設計。Job Health / Cost / Workspace Activity / Errors の固定ダッシュボードを内製しつつ ad-hoc は Metabase 等に逃がす方針

## Future Improvements（別ファイル）

- [../../docs/llm-worker-tools.md](../llm-worker-tools.md) — semantic_search の two-stage re-rank、PDF/画像対応

## 可観測性・ロギング

- [logging.md](logging.md) — 追加すべきログ一覧（P1〜P3）
- [log-viewer.md](log-viewer.md) — log-viewer サブモジュール設計（Logger + JobLogViewer コンポーネント）

## テスト・品質

- [tool-calling-tests.md](tool-calling-tests.md) — LLM エージェントが各ツールを正しく呼び出せているかを確認するテストの追加
- [gcs-put-upload-test.md](gcs-put-upload-test.md) — 署名付き URL を使った GCS PUT アップロードの統合テスト追加
