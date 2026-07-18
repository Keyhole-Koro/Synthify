# 依存関係アーキテクチャ・ガイドライン

## 概要

Synthify プロジェクトにおける各モジュールの責務、依存の方向、および契約（Contract）の管理方法を定義します。本アーキテクチャは「変更しやすく、壊れにくく、生成物がずれにくい」状態を維持することを目的としています。

---

## 1. 依存の基本原則

### 1.1 一方向依存の徹底（Dependency Rule）

依存は常に「外側から内側へ」向かう必要があります。

*   **内側（Core/Contract）:** `internal/gen`, `apps/web/src/gen/proto`
*   **横断基盤（Platform）:** `internal/platform`
*   **外側（Implementation/App）:** `apps/api`, `apps/worker`, `apps/monitor`, `apps/web`

**禁止事項:**
*   `internal/platform` から `apps/api` や `apps/worker` を import すること。
*   アプリケーション間 (`apps/api` と `apps/worker` 等) で直接 import しあうこと。

### 1.2 契約と実装の分離

共通契約（`internal/gen`）は「何をするか（RPC Interface）」を定義し、具体的なドメインロジックや実装は各アプリケーション層に閉じ込めます。ドメイン知識が漏れ出す巨大な共通パッケージは作成しません。

---

## 2. プロジェクトの物理構造と実依存

### 2.1 プロジェクト構造マップ

プロジェクトは以下の階層構造で管理されています。

```text
[Synthify root]
├─ contracts/connectrpc
│  └─ Connect / gRPC proto source of truth
│
├─ internal/
│  ├─ gen/        # Go generated proto / Connect code
│  └─ platform/   # 変化の少ない横断基盤 (applog, observability, util等)
│
├─ apps/
│  ├─ api/
│  │  └─ internal/ # API 固有の domain, service, repository, bootstrap, middleware
│  ├─ worker/
│  │  └─ pkg/worker/ # worker 固有の logic (eval から参照するため pkg 配下)
│  ├─ monitor/
│  ├─ web/
│  │  └─ src/gen/proto # Web 向けの TS generated proto code
│  └─ eval/        # worker の評価・テストツール (worker/pkg を参照)
```

### 2.2 コード上の実依存関係

#### Go 系
```text
api --------> internal/gen, internal/platform
worker -----> internal/gen, internal/platform
eval -------> apps/worker/pkg/worker/..., internal/platform
monitor -> (自己完結したコードを使用)
```

---

## 3. モジュール別の責務

### 3.1 `internal/gen`

API 契約の唯一のソース (`contracts/connectrpc`) から生成された Go コードを管理します。
*   ビジネスロジックは持ちません。

### 3.2 `internal/platform`

変更頻度が低く、ビジネスドメインに依存しない純粋な技術基盤を提供します。
*   **applog:** 構造化ロガー
*   **observability:** New Relic 等のテレメトリ設定
*   **httpmiddleware:** recovery, logger 等の汎用ミドルウェア

### 3.3 `apps/api/internal`

API アプリケーションの全責任を持ちます。
*   **domain:** API 向けのエンティティ・値オブジェクト
*   **repository:** DB 実装 (SQLC 生成物含む)
*   **service:** ユースケース実行
*   **middleware:** Auth, CORS 等の API 固有処理

### 3.4 `apps/worker/pkg/worker`

Worker アプリケーションの全責任を持ちます。
*   LLM 連携、ジョブ実行パイプライン、カスタムツール等の実装。

---

## 4. API 内部レイヤの用語と責務

### 4.1 `service` は Application 層

`apps/api/internal/service` は Application 層として扱います。ここでは、システムが提供するユースケースを完了させるために、次の処理を順番に組み立てます。

*   認証済みユーザーや workspace 権限の確認
*   Repository からの取得
*   Domain の振る舞いや判定の呼び出し
*   Repository への保存
*   通知、ジョブ dispatch、外部サービス呼び出しの調停

Application 層は業務ルールそのものを文字列比較や条件分岐として抱え込まず、Domain が提供する型やメソッドを利用します。

```go
func (s *ItemService) CreateItem(ctx context.Context, workspaceID, label, description, parentID, userID string) (*domain.Item, error) {
    role, err := s.workspaces.GetWorkspaceRole(ctx, workspaceID, userID)
    if err != nil {
        return nil, err
    }
    if !role.CanWrite() {
        return nil, domain.ErrForbidden
    }

    return s.repo.CreateItem(ctx, workspaceID, label, description, parentID, userID)
}
```

この例では、Role の取得と処理順序は Application の責務であり、「どの Role が書き込み可能か」は `role.CanWrite()` という Domain の責務です。

### 4.2 Usecase はレイヤではなく Application の操作単位

Usecase は Application 層と並ぶ別レイヤではありません。Application 層が公開する一つの操作単位を指します。

```text
Application 層 (`service`)
├─ GetItem
├─ CreateItem
├─ ApproveAlias
└─ RejectAlias
```

現在の API では、Usecase ごとに構造体を分けず、対象ごとの Service に複数の Usecase をまとめています。

```go
type ItemUsecase interface {
    GetItem(...)
    CreateItem(...)
    ApproveAlias(...)
    RejectAlias(...)
}
```

したがって、このプロジェクトでは次の語彙を使用します。

*   **Application 層:** `service` package 全体
*   **Application Service:** `ItemService`, `WorkspaceService` など
*   **Usecase:** Application Service が公開する各操作
*   **Usecase interface:** Handler が依存する Application 層の公開 API

### 4.3 `domain` は業務上の意味と判断を持つ

`apps/api/internal/domain` には、業務上の意味を持つ型と、その型だけで判断できるルールを置きます。

*   Entity
*   Value Object
*   enum 相当の型
*   状態遷移
*   業務上の可否判定
*   Domain error
*   複数の Domain object にまたがる純粋な Domain Service / Policy

次のような条件が Service に増えた場合は、Domain へ移すことを検討します。

```go
// Avoid: Application 層が Role の詳細を知っている
if role == "owner" || role == "editor" {
    // ...
}

// Prefer: Domain が業務判断を表現する
if role.CanWrite() {
    // ...
}
```

Domain は Connect RPC、SQLC、PostgreSQL、Stripe、GCS などの技術詳細を import しません。

### 4.4 Repository の契約と実装

Repository は独立したレイヤ名ではなく、永続化を抽象化する役割です。現在の物理配置は次のとおりです。

```text
repository/
├─ interfaces.go  # Application が依存する契約
├─ postgres/      # PostgreSQL / SQLC 実装
└─ mock/          # テスト実装
```

Application Service は `repository` の interface に依存し、`repository/postgres` の具体実装を直接生成しません。

### 4.5 DI と `bootstrap`

依存性注入（DI）は、必要な依存を外部から渡す設計です。具体実装を選択して接続する処理は、Composition Root である `apps/api/internal/bootstrap` に集約します。

```go
itemRepo := postgres.NewItemRepository(db)
workspaceRepo := postgres.NewWorkspaceRepository(db)

itemService := service.NewItemService(service.ItemServiceDeps{
    Repo:       itemRepo,
    Workspaces: workspaceRepo,
    Logger:     logger,
})

itemHandler := handler.NewItemHandler(itemService)
```

Service 内部で DB 接続や PostgreSQL Repository を直接生成してはいけません。

```go
// Avoid
func NewItemService() *ItemService {
    db := openDatabase()
    repo := postgres.NewItemRepository(db)
    return &ItemService{repo: repo}
}
```

ただし、すべての `New` 呼び出しを `bootstrap` に置く必要はありません。Domain object、Request、DTO、処理途中の値など、具体実装の選択を伴わない生成は利用箇所に置けます。

```text
具体的な依存実装を選び、接続する  -> bootstrap
Domain object や値を生成する        -> domain / service 内でも可
```

### 4.6 API 内部の依存方向

```text
handler --------> service --------> domain
                     |
                     +-----------> repository interface

repository/postgres -------------> PostgreSQL / SQLC
infrastructure -------------------> Stripe / GCS / Cloud Tasks
bootstrap ------------------------> 具体実装を生成して全体を接続
```

基本ルールは次のとおりです。

*   Handler は Service の Usecase interface に依存する。
*   Service は Domain と Repository interface に依存する。
*   Domain は Handler、Service、Repository 実装、Infrastructure に依存しない。
*   Repository 実装と Infrastructure は、必要に応じて内側の契約や Domain 型に依存する。
*   `bootstrap` だけが具体実装を知り、アプリケーション全体を組み立てる。

---

## 5. 開発ワークフロー

### 5.1 API 変更の手順

1.  `contracts/connectrpc/` 内の `.proto` ファイルを編集。
2.  ルートディレクトリで `buf generate` を実行。
3.  `internal/gen` と `apps/web/src/gen/proto` が更新されたことを確認。
4.  コンパイルエラーを各アプリで修正。

### 5.2 ドメイン知識の共有

API と Worker で同じデータベーステーブルを参照する場合でも、コード上のドメイン型や Repository 実装は共有（DRY）せず、それぞれのアプリケーション配下に定義・実装します。これにより、一方の変更が意図せず他方に影響を与えることを防ぎます。
