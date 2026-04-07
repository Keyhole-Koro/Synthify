# Proto Draft

## Purpose

このディレクトリには `Go + Connect RPC` 前提の `Protocol Buffers` 叩き台を配置する。

## Layout

- [synthify/graph/v1/common.proto](synthify/graph/v1/common.proto)
- [synthify/graph/v1/graph_types.proto](synthify/graph/v1/graph_types.proto)
- [synthify/graph/v1/user.proto](synthify/graph/v1/user.proto)
- [synthify/graph/v1/workspace.proto](synthify/graph/v1/workspace.proto)
- [synthify/graph/v1/billing.proto](synthify/graph/v1/billing.proto)
- [synthify/graph/v1/document.proto](synthify/graph/v1/document.proto)
- [synthify/graph/v1/graph.proto](synthify/graph/v1/graph.proto)
- [synthify/graph/v1/node.proto](synthify/graph/v1/node.proto)
- [synthify/graph/v1/job.proto](synthify/graph/v1/job.proto)
- [synthify/graph/v1/tool.proto](synthify/graph/v1/tool.proto)
- [synthify/graph/v1/monitoring.proto](synthify/graph/v1/monitoring.proto)

## Design Policy

- package は `synthify.graph.v1` とする
- service は用途ごとに分離し、1ファイル1service を原則とする
- 各 message / enum は所属ドメインの proto ファイルが所有する
- `common.proto` は複数ドメインをまたぐ共有型のみを保持する（`DocumentChunk` 等）
- `graph_types.proto` は `Node`, `Graph` および関連 enum を保持し、service は定義しない
- `graph.proto` は traversal RPC と `PathEvidenceRef` / `GraphPath` を保持する
- package を domain ごとに分割せず、初期は単一 package のまま運用する
- frontend が `React Flow` に直接マップしやすい message 形状を優先する
- 長時間処理は unary RPC で閉じず、job 起動と status 参照に分割する
- breaking change は `synthify.graph.v2` を新設して吸収する
- `node.proto` は node 種別別 API ではなく、`EntityRef` を受ける `GetGraphEntityDetail` で詳細取得を抽象化する

## Notes

- 実ファイル upload は RPC 本体に載せず、`GetUploadURL` で発行した署名付き URL 経由で行う
- `buf` の導入は後続タスクとし、ここでは `.proto` の契約を先に固定する
