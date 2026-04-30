# forceReprocess パラメータが無視されている

## 場所
`api/internal/service/document.go:59` — `StartProcessing()`

## 問題
```go
_ = forceReprocess
```
再処理フラグが完全に無視されており、常に新規処理として扱われる。再処理時の既存 items の扱いとも密接に関係する（[tree-lifecycle-multi-document.md](tree-lifecycle-multi-document.md) 参照）。

## 修正方針
`GenerateExecutionPlan` の signals に `force_reprocess` を渡し、プランナーが既存 items の削除ステップを追加するかどうかの判断に使う。
