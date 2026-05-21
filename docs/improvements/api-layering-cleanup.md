# API レイヤリング整理 (overview)

apps/api のレイヤード構成 (Handler → Service → Repository) に生じている責務漏れ・Fat Interface・実装スタイルのばらつきを段階的に解消する。

## 現状の課題

実コードを精査した結果、以下が確認できた。

1. **Handler が Repository を直接呼んでいる箇所が約 20 箇所**
   `handler/tree.go`, `handler/job.go`, `handler/document.go`, `handler/workspace.go`, `handler/item.go` で `h.repo.*` / `h.workspaces.*` / `h.documents.*` 等が直接叩かれている。
   - ただし「単純な読み取りまで Service 経由にする」のは Anemic Service を量産するだけで反対。**ビジネスロジックを含むものだけ** Service に上げる。

2. **Tree ドメインに Service が存在しない**
   `service/tree.go` が無く、`FindPaths` のような調整ロジック (tree 取得 → path 検索 → evidence 整形) が handler に直書きされている。

3. **`DocumentRepository` の Fat Interface (32 メソッド)**
   Documents / Chunks / Files / Jobs / ApprovalRequests / Logs / ToolCalls / Embeddings が一つの interface に同居している。mock の差し替えが粗く、変更影響範囲が無駄に広い。

4. **Handler の Service 依存が混在**
   `BillingHandler` のみ `service.BillingUsecase` (interface) に依存、他は `*service.XxxService` (具象) に依存。差し替えのテスト容易性に差がある。

5. **認可ロジック (`authorize*`) が handler パッケージにある**
   `handler/authz.go` で repository を直接叩いている。これはビジネスルール寄りなので Service に寄せる余地がある。ただし `connect.NewError` を返す現状を踏まえると単純な引っ越しでは済まない (domain error を介する必要)。

## 設計方針

- **Anemic Service を作らない**: 1行のリポジトリ呼び出しを Service でラップするだけの薄い層は作らない。ビジネスロジックが伴う操作だけを Service に置く。
- **interface 分割は ISP (Interface Segregation) に従う**: 利用者ごとに必要なメソッドだけ持つ interface を定義する。Postgres Store はそれらをまとめて満たす形を維持。
- **段階的に進める**: 一度に全部やらない。各 PR が独立してテスト pass する単位に分けて、レビューと rollback を可能にする。

## ロードマップ (PR 単位)

順序は依存関係に従う。各 PR の詳細はリンク先を参照。

1. [api-document-repository-split.md](api-document-repository-split.md) — `DocumentRepository` を5つの interface に分割 (最優先・他の前提)
2. [api-tree-service.md](api-tree-service.md) — `TreeService` を新設して `FindPaths` 等の調整ロジックを移動
3. [api-service-interfaces.md](api-service-interfaces.md) — `DocumentUsecase` / `WorkspaceUsecase` / `ItemUsecase` を定義し、handler を interface 依存に統一
4. [api-authz-to-service.md](api-authz-to-service.md) — 認可ロジックを Service に移動 (domain.ErrForbidden を介す)

## 範囲外 (明示)

- **Handler の Repository 直接呼び出しを 0 にする** のは方針として採らない。単純な read-only handler は handler 内に repository call を残す。
- **`*Service` を `*Usecase` に rename する** ような表面的な変更は PR 3 で interface 名としてのみ導入。具象クラスの名前は変更しない。
- worker / log-viewer / monitor 等の他アプリのレイヤリング整理は別タスク。

## 関連ドキュメント

- `apps/api/internal/repository/interfaces.go`
- `apps/api/internal/handler/authz.go`
