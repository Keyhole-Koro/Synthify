# Edge 削除タスク

paper-in-paper はツリー構造を `Paper.parentId` / `Paper.childIds` で表現するため、Synthify のフロントエンドでは `Edge` は `parentId`/`childIds` を組み立てるための中間情報として使われた後に捨てられている。概念として不要なので削除したい。

## 現状

### バックエンド

`GetGraph` RPC のレスポンス (`Graph` proto) は `nodes: Node[]` と `edges: Edge[]` を返す。
`Node` proto は `parentId`/`childIds` を持たず、ツリー構造はエッジから導出する設計になっている。

### フロントエンド (`web/src/features/graph/`)

`buildPaperMapFromGraph(nodes, edges)` がエッジを走査して `parentMap`/`childMap` を作り、各 `Paper` の `parentId`/`childIds` に変換する。
`findRootNodeId(nodes, edges)` と `findUnplacedNodeIds(nodes, edges)` も同様にエッジを参照している。

`buildPaperMapFromSubtree(subtree)` も `subtree.edges` から同じ変換を行っている。

## やること

### 1. proto / バックエンド

`Node` メッセージに `parent_id` と `child_ids` フィールドを追加し、`GetGraph` および `GetSubtree` のレスポンスでサーバー側が値を埋めて返すようにする。
`Edge` を返す必要がなくなったら `Graph` proto から `edges` フィールドを削除する。

### 2. フロントエンド

proto 変更後、以下を順に対応する。

- `buildPaperMapFromGraph(nodes, edges)` の `edges` 引数を削除し、`node.parentId`/`node.childIds` を直接使うよう書き換える
- `buildPaperMapFromSubtree` も同様に `subtree.edges` の走査を削除する
- `findRootNodeId` と `findUnplacedNodeIds` から `edges` 引数を削除する
- `useWorkspaceGraph.tsx` の `findRootNodeId(graph.nodes, graph.edges)` 呼び出しを `findRootNodeId(graph.nodes)` に変える
- `ApiEdge` / `SubtreeEdge` の型定義と export を `graph/api.ts` から削除する

### 3. 確認

`npx tsc --noEmit` がエラーなしで通ること。
