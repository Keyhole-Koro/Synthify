# ADK エージェント実行エラーが握りつぶされている

## 場所
`worker/pkg/worker/worker.go:185` — `processDocument()`

## 問題
```go
_ = w.runAgentBestEffort(ctx, req, rawText)
```
`runAgentBestEffort` のエラーを完全に無視しており、エージェント実行が失敗してもジョブが成功扱いになる可能性がある。"BestEffort" という名前がついているが、エラーをログにも残していない。

## 修正方針
エラーを `log.Printf` で最低限記録する。ジョブステータスを `failed` に更新するかどうかはエラーの種別（タイムアウト/モデルエラー/ツールエラー）に応じて判断する。
