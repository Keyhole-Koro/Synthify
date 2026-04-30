# synthesizeItems が deterministic stub でLLMを使っていない

## 場所
`worker/pkg/worker/worker.go` — `synthesizeItems`

## 問題
`synthesizeItems` はchunkのテキストを機械的にアイテムに変換するだけで、LLMによる意味解析・構造化を行っていない。結果として生成されるナレッジツリーの品質が低く、ドキュメントの構造をそのまま写すだけになる。

## 修正方針
ADKエージェント（`runAgentBestEffort`）またはLLMクライアントを使い、chunkから概念・関係・階層を抽出させる。`GenerateStructured` でスキーマを指定して `[]SynthesizedItem` を直接得る形が最も安定する。
