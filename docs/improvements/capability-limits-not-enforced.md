# JobCapability の上限が実際には強制されていない

## 場所
`worker/pkg/worker/worker.go`、`worker/pkg/worker/agents/`

## 問題
`JobCapability` に `MaxLlmCalls`、`MaxToolRuns`、`MaxItemCreations` フィールドが定義されているが、実行中にカウントも強制もされていない。ジョブが割り当てリソースを超過しても止まらない。

## 修正方針
`BaseContext` にカウンターを持たせ、各ツール実行時にインクリメント・上限チェックを行う。超過時はエラーを返してジョブを停止させる。
