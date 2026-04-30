# Refactor 001: チャンク分割ロジックの共有化

## 現状
ドキュメントをセマンティックな単位に分割するロジック（`buildChunks`, `isHeadingLine`, `splitSections` 等）が `worker/pkg/worker/worker.go` および `worker/pkg/worker/tools/chunking.go` に直接記述されています。

## 課題
- API側で「アップロード前に分割結果をプレビューする」などの機能を実装する場合、同じロジックを再実装する必要がある。
- 分割ルール（見出しの判定基準など）を変更した際、複数の場所を修正しなければならない。

## 解決策
- `shared/domain` または新設する `shared/pipeline` 等にチャンク分割ロジックを移動。
- `api` と `worker` の両方から同じ分割エンジンを利用可能にする。
