# Store interface narrowing (handler / service ごとに必要な repository だけを受け取る)

## 背景

`packages/shared/app/bootstrap.go` の `Store` interface は God Interface
だった。現状は `repository/interfaces.go` のドメイン別 sub-interface
(`AccountRepository`, `WorkspaceRepository`, `DocumentRepository`,
`TreeRepository`, `ItemRepository`, `UsageRepository`,
`CheckpointRepository`, `JobLogWriter`) を embed する集約 interface に
リファクタ済み。これにより `Store` の宣言自体は 10 行程度に縮んだが、
**handler や service が依然として `Store` を丸ごと受け取っている** という
構造上の問題は残っている。

## 問題

- handler / service の依存が不必要に広い。`WorkspaceHandler` が tree や
  job のメソッドにもアクセスできてしまう。
- テストで `Store` 全体を mock する必要があるため、関心外のメソッドにも
  振る舞いを与えないといけない場合がある。
- 「この層は何のために存在するのか」がシグネチャから読めない。

## 望ましい姿（Plan A）

各 handler / service は自分が必要とする repository interface だけを
受け取る。例:

```go
// Before
func NewWorkspaceHandler(svc *WorkspaceService, billing BillingUsecase, store app.Store) *WorkspaceHandler

// After
func NewWorkspaceHandler(
    svc *WorkspaceService,
    billing BillingUsecase,
    repo repository.WorkspaceRepository,
) *WorkspaceHandler
```

`app.Store` は bootstrap / wiring 層でだけ参照され、各 component を
組み立てるときに必要な interface へ降格させる。

## 移行方針

1. 影響範囲の小さい handler (workspace, item, document) から着手。
   - コンストラクタ引数を分割 interface に置き換える。
   - 呼び出し側 (`cmd/server/main.go`) は `app.Store` から渡すだけなので
     embed のおかげで型互換性は維持される。
2. service 層も同様に置き換える。
3. job 系は `DocumentRepository` がやや太いので、必要なら更に分割
   （`JobRepository`, `JobApprovalRepository` などへ切り出し）する。
4. 全 handler/service が narrow interface になったら、`app.Store` を
   bootstrap 専用の集約 interface としてのみ残す。

## 非目標

- `Store` という名前自体の廃止。集約 interface は bootstrap の都合上
  残してよい。
- 一気に全層リファクタすること。Plan A は段階的に進める。

## 関連

- 完了済み: `Store` interface を embed ベースに集約化
  (`packages/shared/app/bootstrap.go`)。
