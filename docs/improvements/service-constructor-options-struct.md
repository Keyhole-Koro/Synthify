# Service コンストラクタを Options struct 化する

`service.NewDocumentService(store, store, store, store, store, store, sourceURL, metadata, objectStore, dispatcher, notifier, logger, nrApp)` のような「同じ store が複数 interface を実装しているため呼び出し側で同じ識別子が何度も並ぶ」シグネチャを解消する。

## 現状

[apps/api/internal/service/document.go](../../apps/api/internal/service/document.go) の `NewDocumentService` は positional に 12 引数 + 可変長 1 を取る。Postgres / mock の `Store` がほぼ全ての repository interface を一つの struct で実装しているため、main.go から見るとこうなる:

```go
service.NewDocumentService(store, store, store, store, store, store, sourceURLBuilder, objectMetadata, objectStore, dispatcher, notifier, appLogger, nrApp)
```

問題:
- 引数の意味が呼び出し側からほぼ読めず、順序を 1 個入れ替えても build が通る組み合わせがあり危険。
- 新しい依存を足すたびに全テストの `nil` パディングを足す必要がある (今回の objectStore 追加で 9 箇所の test を編集した)。
- 同じパターンが `WorkspaceService` / `BillingService` / `UserService` などにも波及している。

## 方針案

```go
type DocumentServiceDeps struct {
    Repo             repository.DocumentRepository
    Jobs             repository.JobRepository
    LifecycleRepo    joblifecycle.Repository
    Workspaces       repository.WorkspaceRepository
    Tree             repository.TreeRepository
    Transactor       repository.Transactor
    SourceURLBuilder repository.DocumentSourceURLBuilder
    ObjectMetadata   ObjectMetadataFetcher
    ObjectStore      repository.DocumentObjectStore
    Dispatcher       WorkerDispatcher
    Notifier         jobstatus.Notifier
    Logger           *slog.Logger
    NRApp            *newrelic.Application
}

func NewDocumentService(deps DocumentServiceDeps) *DocumentService { ... }
```

呼び出し側:

```go
documentSvc := service.NewDocumentService(service.DocumentServiceDeps{
    Repo: store, Jobs: store, LifecycleRepo: store,
    Workspaces: store, Tree: store, Transactor: store,
    SourceURLBuilder: sourceURLBuilder,
    ObjectMetadata:   objectMetadata,
    ObjectStore:      objectStore,
    Dispatcher:       dispatcher,
    Notifier:         notifier,
    Logger:           appLogger,
    NRApp:            nrApp,
})
```

## やること

1. `DocumentServiceDeps` を service パッケージに追加し、`NewDocumentService` を新形式に置き換える。`nrApp` の variadic は廃止。
2. main.go の wiring を書き換える。
3. test (`service/document_test.go`, `handler/auth_flow_test.go`, `handler/document_upload_integration_test.go`) の 9 箇所を更新する。`nil` パディングは消える。
4. 同じパターンの `BillingService` / `WorkspaceService` / `UserService` / `ItemService` / `TreeService` も並行で揃える。バラバラだと意味が薄い。

## やらないこと

- repository 側の interface 分割 (`DocumentRepository` を細かく切る等) はこの PR の対象外。あくまでコンストラクタの取り回しの問題に絞る。
- Functional options (`WithFoo(...)` 形式) は採用しない。Synthify では「依存をすべて明示する」方が読みやすく、optional な依存はほぼ無いため。

## きっかけ

[../learn/upload-pipeline-hardening.md](../learn/upload-pipeline-hardening.md) の作業で `NewDocumentService` に `ObjectStore` を 1 つ足したら、`nil` パディングの追加だけでテストを 9 箇所触ることになった。これ以上引数が増えると positional の限界。
