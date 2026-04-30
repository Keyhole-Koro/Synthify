# Worker ツール群の LLM 実装（完了）

## 場所
- `worker/pkg/worker/tools/process/synthesis.go`
- `worker/pkg/worker/tools/process/merging.go`
- `worker/pkg/worker/tools/process/briefing.go`
- `worker/pkg/worker/tools/process/critique.go`

## 対応済み

全ツールを `llm.GenerateStructuredSimple` を使った LLM 実装に切り替えた。
詳細設計・プロンプト・フォールバック方針は [process-tools-llm-implementation.md](process-tools-llm-implementation.md) を参照。
