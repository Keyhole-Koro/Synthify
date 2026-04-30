# Orchestrator.ProcessDocument がプレースホルダー実装

## 場所
`worker/pkg/worker/agents/orchestrator.go:143` — `ProcessDocument()`

## 問題
```go
return "Orchestration started (ADK transition in progress)", nil
```
実際の処理を何も行わず固定文字列を返すだけ。ADK への移行途中で残されたプレースホルダー。

## 修正方針
現状の worker フローでは `runAgentBestEffort` が直接 ADK を呼んでいるため、`Orchestrator.ProcessDocument` は呼ばれていない可能性がある。実際に呼ばれているか確認した上で、不要なら削除、必要なら ADK runner への委譲を実装する。
