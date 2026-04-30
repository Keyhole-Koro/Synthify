---
# mock.Store の IsWorkspaceAccessible が常に true を返す

## 場所
`shared/repository/mock/store.go` — `IsWorkspaceAccessible`

## 問題
```go
func (s *Store) IsWorkspaceAccessible(ctx context.Context, wsID, userID string) bool { return true }
```
アクセス制御が常に許可されるため、以下のテストが全て誤パスまたは誤フェイルしている。

- `TestAuthorizeWorkspace_NotMember_ReturnsPermissionDenied` — 拒否されるべきが通ってしまう
- `TestAuthorizeDocument_*` 系 — 同上
- `TestGetWorkspace_NonMember_ReturnsErrNotFound` — ErrNotFound が返らない

## 修正方針
ワークスペース作成時に `ownerID → []wsID` のマップを持ち、`IsWorkspaceAccessible` でそのユーザーが所有または招待されているか確認する。
