# Worker ツール群が簡易実装のまま

## 場所
- `worker/pkg/worker/tools/synthesis.go` — `NewSynthesisTool()`
- `worker/pkg/worker/tools/merging.go` — `NewMergeTool()`
- `worker/pkg/worker/tools/briefing.go` — `NewBriefTool()`
- `worker/pkg/worker/tools/critique.go` — `NewCritiqueTool()`

## 問題

**synthesis**: `DocumentBrief` と `Glossary` を受け取っているが使っていない。チャンク → アイテム変換が機械的で LLM を使っていない。

**merging**: 常に最初のアイテム ID を返すだけ。実際のマージ判定ロジックがない。

**briefing**: テキストを単純に結合するだけ。ドキュメントの要約・分析を行っていない。

**critique**: `"stub"` 文字列を含むかどうかの静的チェックのみ。LLM による品質評価が未実装。

## 修正方針
いずれも LLM（`functiontool` 経由の structured output）を使って実装する。`synthesizeItems` の LLM 化（[synthesize-items-deterministic-stub.md](synthesize-items-deterministic-stub.md)）と合わせて対応する。
