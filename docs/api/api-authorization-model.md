# API authorization model

このドキュメントは、Synthify API の handler がどのように「ログイン済みユーザーが対象データへアクセスできるか」を確認しているかを説明する。

ここでの authorization は「権限を付与する」処理ではなく、リクエスト時のアクセス権チェックを指す。

## 全体像

API のデータはおおむね次の親子関係で扱われる。

```text
account / user
  -> workspace
      -> document
          -> processing job
      -> tree item
```

基本ルールは単純で、ユーザーが対象 workspace にアクセスできるなら、その workspace 配下の document / job / tree item も操作できる、という設計になっている。

そのため、多くの handler は最終的に `WorkspaceRepository.IsWorkspaceAccessible` を呼んでいる。

## 認証と認可

このコードベースでは認証と認可を分けて考える。

認証:

- リクエスト元が誰かを確認する
- 実装上は middleware が `context.Context` に `AuthUser` を入れる
- handler は `currentUser(ctx)` で取り出す

認可:

- そのユーザーが対象 resource を見たり変更したりできるかを確認する
- 実装上は `authorizeWorkspace` / `authorizeDocument` / `authorizeItem` を使う

該当コードは [api/internal/handler/authz.go](/home/unix/Synthify/api/internal/handler/authz.go) にある。

## `currentUser`

`currentUser(ctx)` は context から認証済みユーザーを取り出す。

ユーザーが存在しない、または user ID が空の場合は `Unauthenticated` を返す。

```go
user, err := currentUser(ctx)
if err != nil {
    return nil, err
}
```

この時点では「ユーザーが誰か」だけが分かる。特定 workspace や document へのアクセス権はまだ確認していない。

## `authorizeWorkspace`

workspace ID がリクエストに直接含まれる API は、まず workspace へのアクセス権を確認する。

例:

- `ListDocuments(workspace_id)`
- `CreateDocument(workspace_id, ...)`
- `GetTree(workspace_id)`
- `FindPaths(workspace_id, ...)`

流れは次の通り。

```text
1. currentUser(ctx) で user ID を取得
2. WorkspaceRepository.IsWorkspaceAccessible(workspaceID, userID) を呼ぶ
3. false なら PermissionDenied
```

## `authorizeDocument`  

document ID しかリクエストに含まれない API では、document から workspace ID を逆引きしてから workspace access を確認する。

例:

- `GetDocument(document_id)`
- `StartProcessing(document_id)`
- job 系 API の多く

流れは次の通り。

```text
1. DocumentRepository.GetDocument(documentID) で document を取得
2. document がなければ NotFound
3. expectedWorkspaceID が指定されていて一致しなければ PermissionDenied
4. document.WorkspaceID を使って authorizeWorkspace を呼ぶ
```

job 系 API で document authorization が必要になるのは、job が document に紐づく resource だから。

たとえば `GetJobStatus(job_id)` は workspace ID を受け取らない。そこで handler は次の順に確認する。

```text
1. job_id から processing job を取得
2. job.DocumentID を見る
3. document を取得して WorkspaceID を見る
4. current user がその workspace にアクセスできるか確認する
```

このため `JobHandler` は job を読む repository だけでなく、document を読む repository と workspace access を見る repository も必要になる。

## `authorizeItem`

tree item 系 API では item の存在確認をしてから workspace access を確認する。

現在の実装は次の形。

```text
1. ItemRepository.GetItem(itemID) で item の存在を確認
2. request に含まれる workspaceID に対して authorizeWorkspace を呼ぶ
```

注意点として、現状の `authorizeItem` は item 自身の workspace ID と request の workspace ID の一致までは確認していない。item model / repository 側の情報設計次第では、ここは将来的に強化候補になる。

## なぜ `store` を複数渡しているのか

`api/cmd/server/main.go` では、次のような呼び出しが出てくる。

```go
handler.NewJobHandler(store, store, store)
```

これは `store` が複数の repository interface を実装しているため。

`JobHandler` は概念上、次の依存を持つ。

```text
job repository       -> job / plan / approval / mutation log を読む
document repository  -> job に紐づく document を読む
workspace repository -> document の workspace に current user がアクセスできるか確認する
```

現在はこれらを同じ concrete object である `store` が全部実装している。そのため呼び出し側では同じ値を複数回渡している。

ただし、見た目としては分かりにくい。特に job 操作も document 操作も `DocumentRepository` に含まれているため、`NewJobHandler(store, store, store)` は意図が読み取りづらい。

改善するなら、constructor を役割ごとに整理して次のようにするのが自然。

```go
func NewJobHandler(
    documents repository.DocumentRepository,
    workspaces repository.WorkspaceRepository,
) *JobHandler
```

この形なら呼び出し側は次のようになる。

```go
handler.NewJobHandler(store, store)
```

さらに読みやすくするなら、`store` を直接並べるのではなく、API dependencies をまとめた構造体を作る選択肢もある。

```go
type HandlerDeps struct {
    Workspaces repository.WorkspaceRepository
    Documents  repository.DocumentRepository
    Items      repository.ItemRepository
    Trees      repository.TreeRepository
}
```

## Handler と service の役割

このコードベースでは、概ね次の分担にすると理解しやすい。

handler:

- request validation
- current user の取得
- authorization
- proto/domain mapper の呼び出し
- Connect error への変換

service:

- 複数 repository をまたぐ業務処理
- side effect を伴う workflow
- dispatcher / notifier / upload URL generator など外部処理の組み合わせ

そのため、repository をただ呼ぶだけの service method は削除対象になりやすい。一方で `DocumentService.StartProcessing` のように tree 作成、job 作成、worker dispatch、notifier を組み合わせる処理は service に残す価値がある。

## エラーコードの考え方

現在の helper はおおむね次の方針で Connect error を返す。

```text
ログインしていない             -> Unauthenticated
必須 ID が空                   -> InvalidArgument
resource が存在しない          -> NotFound
resource はあるがアクセス不可  -> PermissionDenied
```

ただし、存在確認とアクセス権チェックの順序によって、外部から resource の存在が推測できる場合がある。公開 API として厳密に隠したい resource では、`NotFound` と `PermissionDenied` の使い分けを再設計する余地がある。

## 読む順番

体系的に追うなら、次の順で読むとよい。

1. [shared/repository/interfaces.go](/home/unix/Synthify/shared/repository/interfaces.go) で repository interface を見る
2. [api/internal/handler/authz.go](/home/unix/Synthify/api/internal/handler/authz.go) で authorization helper を見る
3. [api/internal/handler/workspace.go](/home/unix/Synthify/api/internal/handler/workspace.go) で workspace 直指定 API を見る
4. [api/internal/handler/document.go](/home/unix/Synthify/api/internal/handler/document.go) で document から workspace を逆引きする流れを見る
5. [api/internal/handler/job.go](/home/unix/Synthify/api/internal/handler/job.go) で job から document を経由して workspace access を確認する流れを見る

