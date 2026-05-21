# PR 2: TreeService の新設

親: [api-layering-cleanup.md](api-layering-cleanup.md)

## 現状の課題

Tree ドメインだけ Service が存在しない。`apps/api/internal/handler/tree.go` が `TreeRepository` / `DocumentRepository` / `WorkspaceRepository` を直接受け取り、ビジネスロジックを handler 内に書いている。

特に問題のあるメソッド:

- **`FindPaths`** — workspace の tree を取得 → path 検索 → evidence 整形、という調整ロジックを handler が直接やっている。これは Service の典型的な仕事。
- **`GetSubtree`** — `items[0].WorkspaceID != wsID` で workspace 境界をチェックしている。本来は workspace 認可と並列の概念で、ビジネスルール側に置くべき。

## 改善目標

`apps/api/internal/service/tree.go` を新規作成し、上記の調整ロジックと境界チェックを移動する。handler は引数の整形と認可だけを残す。

## 実装計画

### Phase 1: TreeService の新設
- `apps/api/internal/service/tree.go` を作成。
- 必要な依存: `TreeRepository` (PR 1 で分割される場合あり, 分割されなくても OK)、`WorkspaceRepository`。
- メソッド:
  - `GetTree(ctx, workspaceID) ([]*domain.Item, error)`
  - `GetSubtree(ctx, workspaceID, itemID, maxDepth int) ([]*domain.Item, error)` — workspace 境界チェックを含む。違反時は `domain.ErrForbidden` を返す (PR 4 で導入予定の error 型)。
  - `FindPaths(ctx, workspaceID, sourceItemID, targetItemID string, maxDepth, limit int) (items, paths, error)` — tree 取得と path 検索を内部でまとめる。

### Phase 2: handler を Service 経由に切り替え
- `TreeHandler` のコンストラクタを `TreeService` を受け取る形に変更する。
- `cmd/server/main.go` の wiring を更新。
- handler 側は proto 変換と認可のみを残す。

### Phase 3: テスト
- `service/tree_test.go` を新規作成し、Service レベルの単体テストを追加。
- 既存の handler テスト (もしあれば) は維持。

## 範囲外

- Item や Document の Service は今回触らない。
- Tree の repository 側は触らない (`TreeRepository` のメソッド構成は維持)。

## 完了条件

- `service/tree.go` と `service/tree_test.go` が存在する。
- `TreeHandler` が `TreeRepository` を直接持っていない。
- `go build` / `go test ./apps/api/...` が pass する。

## 関連

- PR 1 ([api-document-repository-split.md](api-document-repository-split.md)) と独立して進められる。並行実装可能。
- PR 4 ([api-authz-to-service.md](api-authz-to-service.md)) で `domain.ErrForbidden` を導入予定。それまでは `errors.New("...")` でも `connect.NewError` 直接でも可とする (Phase 4 で揃える)。
