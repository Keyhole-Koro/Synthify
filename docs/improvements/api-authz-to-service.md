# PR 4: 認可ロジックを Service 層へ移動

親: [api-layering-cleanup.md](api-layering-cleanup.md)

## 現状の課題

`apps/api/internal/handler/authz.go` に `authorizeWorkspace` / `authorizeDocument` / `authorizeItem` が定義されており、handler パッケージから repository を直接叩いている。

```go
func authorizeWorkspace(ctx, repo repository.WorkspaceRepository, workspaceID string) error {
    userID, err := requireUserID(ctx)
    ...
    if !repo.IsWorkspaceAccessible(ctx, workspaceID, userID) {
        return connect.NewError(connect.CodePermissionDenied, ...)
    }
    return nil
}
```

問題:
- 「このユーザーがこのリソースにアクセスできるか」はビジネスルール。handler に置くと将来「監査ログを残す」「accept-list で例外を許可する」等の拡張が handler パッケージに集積する。
- handler の各メソッドが `authorize*(ctx, repo, ...)` を呼んで、その後 `service.*` を呼ぶ二段構えになっており、Service が「認可済み前提」で動くのか「Service 内で再チェックするのか」が曖昧。

## 改善目標

認可を **Service の責務** にする。handler は引数の整形と Connect エラー変換だけを残す。Service が `domain.ErrForbidden` / `domain.ErrNotFound` を返し、handler の `toError` がそれを `connect.Error` に変換する。

## 実装計画

### Phase 1: domain エラーの追加
`apps/api/internal/domain/errors.go` に以下を追加:

```go
var (
    ErrForbidden       = errors.New("forbidden")
    ErrUnauthenticated = errors.New("unauthenticated")
)
```

`handler/errors.go` の `toError` を拡張し、これらを適切な connect code にマッピング:
- `ErrForbidden` → `connect.CodePermissionDenied`
- `ErrUnauthenticated` → `connect.CodeUnauthenticated`
- `ErrNotFound` → `connect.CodeNotFound` (既存)

### Phase 2: 各 Service に認可を組み込む
PR 3 で導入する `*Usecase` interface のメソッド内で、リソース取得前に認可をチェックする。

例: `WorkspaceUsecase.GetWorkspace(ctx, id, userID)` 内で:
```go
if !s.workspaces.IsWorkspaceAccessible(ctx, id, userID) {
    return nil, domain.ErrForbidden
}
```

対象:
- `WorkspaceUsecase`: 既に `GetWorkspace` 等は `userID` を取って `IsWorkspaceAccessible` チェックを持っているので、整理だけ。
- `DocumentUsecase`: `authorizeDocument` の処理を取り込む。
- `ItemUsecase`: `authorizeItem` の処理を取り込む。
- `TreeUsecase` (PR 2 で作成済み前提): workspace 境界チェックを内部に。

### Phase 3: handler から authorize* を削除
- `handler/authz.go` の `requireUserID` / `requireAuthUser` は残す (これは handler の責務)。
- `authorizeWorkspace` / `authorizeDocument` / `authorizeItem` を削除する。
- 各 handler は `Service` メソッドに `userID` を渡し、戻ってきたエラーを `toError` でラップする。

before:
```go
if err := authorizeWorkspace(ctx, h.workspaces, wsID); err != nil { return nil, err }
ws, err := h.service.GetWorkspace(ctx, wsID, userID)
if err != nil { return nil, toError(err) }
```

after:
```go
ws, err := h.service.GetWorkspace(ctx, wsID, userID)
if err != nil { return nil, toError(err) }
```

### Phase 4: テストの書き直し
- `handler/authz_test.go` から `authorize*` のテストを削除。
- それぞれの Service テストに「未認証ユーザーが拒否される」「他人のリソースが見えない」テストを移植する。

## 範囲外

- middleware の auth (Firebase token 検証) は触らない。
- Admin / IsAdmin 判定 は今回の認可移動の対象外 (handler 内に残す)。

## 完了条件

- `handler/authz.go` に `authorize*` 系の関数が無い。
- 各 Service が認可済みの結果を返す (`domain.ErrForbidden` を返しうる)。
- handler の各メソッドが「Service を呼ぶ → エラーを toError で変換」のシンプルな流れになっている。
- `go build` / `go test ./apps/api/...` が pass する。

## 関連 / 依存

- PR 3 ([api-service-interfaces.md](api-service-interfaces.md)) の完了を前提とする (Service interface に認可を持たせるため)。
- PR 1, 2 とは独立だが、これが最後で一番影響範囲が大きい。
