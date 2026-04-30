# Improvements

既知の問題点・改善候補の一覧。優先度順に並べている。

## P1 — 設計上の問題

- [capability-limits-not-enforced.md](capability-limits-not-enforced.md) — JobCapability のLLM呼び出し上限が実際には強制されていない
- [synthesize-items-deterministic-stub.md](synthesize-items-deterministic-stub.md) — synthesizeItems がdeterministic stubでLLMを使っていない
- [mock-workspace-access-always-true.md](mock-workspace-access-always-true.md) — mock の IsWorkspaceAccessible が常に true でアクセス制御テストが壊れている

## P2 — 仕様ドラフト（実装前に設計が必要）

- [tree-lifecycle-multi-document.md](tree-lifecycle-multi-document.md) — 複数ドキュメント処理時の tree 統合・更新ライフサイクル（Phase 1〜3）

## Future Improvements（別ファイル）

- [../../docs/llm-worker-tools.md](../llm-worker-tools.md) — semantic_search の two-stage re-rank、PDF/画像対応
