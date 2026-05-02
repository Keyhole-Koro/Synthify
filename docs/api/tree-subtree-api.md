# Tree subtree API

`GetSubtree` は、knowledge tree の一部だけを取得するための API。

ある item を root として、その配下の item 群を `max_depth` の範囲で返す。tree 全体を毎回取得せず、画面上で開かれた item の周辺だけを追加で読みたいときに使う。

## Endpoint

Connect RPC ではなく、HTTP endpoint として登録されている。

```go
mux.HandleFunc("GET /tree/subtree", treeHandler.GetSubtreeHTTP)
```

実装は [api/internal/handler/tree.go](/home/unix/Synthify/api/internal/handler/tree.go) の `GetSubtreeHTTP`。

## Request

query parameter で指定する。

```text
GET /tree/subtree?workspace_id=...&item_id=...&max_depth=3
```

parameters:

- `workspace_id`: 必須。アクセス権チェックに使う workspace ID。
- `item_id`: 必須。subtree の root として扱う item ID。
- `max_depth`: 任意。root から何階層下まで返すか。未指定時は `3`。

## Authorization

`GetSubtreeHTTP` は `workspace_id` に対して workspace access を確認する。

流れは次の通り。

```text
1. request context から current user を取得
2. user がなければ 401
3. WorkspaceRepository.IsWorkspaceAccessible(workspace_id, user.ID) を呼ぶ
4. false なら 403
5. TreeRepository.GetSubtree(item_id, max_depth) を呼ぶ
6. 返ってきた root item の `workspace_id` が request の `workspace_id` と一致するか確認する
7. 一致しなければ 403
```

つまり、アクセス可能な workspace ID と別 workspace の item ID を組み合わせた request は拒否される。

## Repository

repository interface は [shared/repository/interfaces.go](/home/unix/Synthify/shared/repository/interfaces.go) にある。

```go
GetSubtree(ctx context.Context, rootItemID string, maxDepth int) ([]*domain.SubtreeItem, error)
```

`rootItemID` が subtree の起点で、`maxDepth` が取得する深さ。

## Response

HTTP JSON として次の形の配列を返す。

```json
[
  {
    "id": "item-root",
    "label": "Root item",
    "level": 0,
    "description": "...",
    "summary_html": "<p>...</p>",
    "has_children": true,
    "parent_id": "",
    "child_ids": ["item-child"]
  }
]
```

fields:

- `id`: item ID。
- `label`: 表示名。
- `level`: subtree 内での階層。
- `description`: item の説明。
- `summary_html`: item の HTML summary。空なら省略される。
- `has_children`: さらに子 item があるか。
- `parent_id`: parent item ID。root では空になることがある。
- `child_ids`: 子 item ID の一覧。

## GetTree との違い

`GetTree` は workspace の tree 全体を取得する RPC。

`GetSubtree` は item を起点にした部分木だけを取得する HTTP endpoint。

```text
GetTree:
  workspace_id -> tree 全体

GetSubtree:
  workspace_id + item_id -> item 配下の部分木
```

フロントエンドで tree を段階的に開く UI では、初期表示に `GetTree`、追加展開に `GetSubtree` を使う構成が考えられる。
