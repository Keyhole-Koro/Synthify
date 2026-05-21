# PR 3: Service 層を interface 化

親: [api-layering-cleanup.md](api-layering-cleanup.md)

## 現状の課題

handler の Service 依存スタイルが揃っていない:

- `BillingHandler` だけ `service.BillingUsecase` (interface) に依存。
- 他 (`DocumentHandler`, `WorkspaceHandler`, `ItemHandler`, `JobHandler`, `UserHandler`) は `*service.XxxService` (具象) に依存。

結果として:
- 一部 handler テストで Service を mock 差し替えできない (具象を全部組み立てる必要)。
- 認可ロジックを Service に移す PR 4 が、Service の差し替え可能性を必要とする。先に interface 化しておくのが筋。

## 改善目標

各 Service に `*Usecase` interface を導入し、handler はその interface に依存する形に統一する。具象クラス (`DocumentService` 等) の名前と実装は変更しない。

## 実装計画

### Phase 1: Usecase interface の定義
各 Service ファイルに、handler が呼ぶメソッドだけを並べた interface を定義する。

- `service/document.go`: `DocumentUsecase`
- `service/workspace.go`: `WorkspaceUsecase`
- `service/item.go`: `ItemUsecase`
- `service/user.go`: `UserUsecase`
- `service/tree.go`: `TreeUsecase` (PR 2 で作成済み前提)
- 既存の `BillingUsecase` はそのまま。

interface には handler から呼ばれているメソッドだけを含める。Service の内部 helper (`upsertUserRow` 等) は含めない。

### Phase 2: handler の依存型を interface に変更
- `WorkspaceHandler.service *service.WorkspaceService` → `service.WorkspaceUsecase`
- 同様に他 handler も書き換える。
- `cmd/server/main.go` の wiring は変更不要 (具象が interface を満たすため)。

### Phase 3: 既存テストの確認
- 既存テストはコンストラクタに具象を渡しているはずなので、interface 化しても影響なし。
- 新たに mock Service を作る作業は、必要が出たときに各 handler テスト側でやる (この PR では作らない)。

## 範囲外

- 具象クラスの rename (`DocumentService` → `documentService` 等) はしない。export しっぱなしで OK。
- `BillingUsecase` の構造は変更しない (既に揃っている)。
- Service の内部実装は触らない。

## 完了条件

- 各 Service ファイルに `*Usecase` interface が定義されている。
- 全 handler が `service.*Usecase` interface に依存している。
- `go build` / `go test ./apps/api/...` が pass する。

## 関連

- PR 4 ([api-authz-to-service.md](api-authz-to-service.md)) の前提として完了している必要がある。
- PR 1, 2 とは独立して進められる。
