# テストマトリクス: `dev_seed_test.go`

このマトリクスは、`dev_seed_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

> **読み方の注意**: 下の「テストケース表」は *既存テストが何を確認しているか* を写したものなので、
> **テストが1件も無いメソッド／分岐はこの表に現れない**。
> 「インターフェース網羅チェック」「依存エラー軸」を併読すること。
> カバレッジ数値は `go test -coverprofile` の実測 (2026-06-12)。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

## インターフェース網羅チェック (`DevSeedUsecase`)

| メソッド | 専用テスト | coverage | 状態 | 未テストの主分岐 |
| --- | --- | --- | --- | --- |
| `SeedWorkspace` | ✅ 1件 | 66.7% | PARTIAL | **`userID == ""` の早期 error**、`GetOrCreateAccount` repo error。 |

内部 helper の実測: `findOrCreateSeedWorkspace` 80.0% (`ListWorkspacesByUser` / `CreateWorkspace` error) /
`seedTree` 86.4% (`GetTreeByWorkspace` error、missing parent error、`CreateItem` error)。

→ idempotency (2回実行で item 非増加) は確認済みだが、**部分 seed 補完** (一部 item だけ存在する
workspace で再 seed して欠損分のみ作る) が未テスト。`seedTree` の `itemByParentTitle` 再利用ロジックの核心。
また `userID == ""` の validation と各 repo の error 伝播も未確認。

## 依存エラー軸 (dependency returns error)

☑=テスト有 / ◐=間接 / ❌=未テスト。

| 経路 | 条件 | 期待 | 状態 |
| --- | --- | --- | --- |
| 入力検証 | `userID == ""` | error | ❌ |
| accounts repo | `GetOrCreateAccount` error | 伝播 | ❌ |
| workspaces repo | `ListWorkspacesByUser` / `CreateWorkspace` error | 伝播 | ❌ |
| tree repo | `GetTreeByWorkspace` error | 伝播 | ❌ |
| items repo | `CreateItem` error (途中失敗) | `created` 件数付きで伝播 | ❌ |
| seed 整合性 | spec の親が未解決 | `missing parent` error | ❌ |
| 部分 seed | 一部 item のみ存在 | 欠損分のみ作成 | ❌ |

→ 正常な idempotency 1件のみ。**異常系・部分 seed・依存エラーが全面未テスト。**

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestDevSeedService_SeedWorkspaceIsIdempotentForUser` | `SeedWorkspace` | idempotency | 同じ firebase user で2回 seed する。 | 1回目は workspace と seed item を作成し、2回目は workspace を再利用して item を追加しない。 | 初回のみ workspace/items が作られる。 | `CreatedWorkspace`, `CreatedItemCount`, `TotalItemCount`, workspace ID 一致 | dev seed の再実行安全性。 | 別 user、既存 workspace が一部壊れている場合、seed node 内容。 | 一部 item 欠損時の補完、別 user で別 workspace 作成。 | PARTIAL |

## 観点別チェックグリッド

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

| テストケース | 正常系 | 異常系 | idempotency | 状態変化 | 永続化副作用 | fixture 内容 | mock repository |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `TestDevSeedService_SeedWorkspaceIsIdempotentForUser` | ☑ | - | ☑ | ☑ | ☑ | ◐ | ☑ |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| idempotency | 同一 user の2回実行は確認済み。 | 一部 seed item 欠損時の再実行、同時実行。 |
| fixture 内容 | 件数のみ確認している。 | seed node の title / parent 構造。 |
| user 境界 | 単一 user のみ確認している。 | 別 user では別 workspace になること。 |

## 境界値チェックマトリクス

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 数値 / count / size

| 対象値 / 条件 | `0` | `1` | typical | `max - 1` | `max` | `max + 1` | negative | huge / overflow | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| seed 実行回数 | - | ☑ | ☑ | - | - | - | - | - | `TestDevSeedService_SeedWorkspaceIsIdempotentForUser` | 3回以上、同時実行。 |
| created item count | ☑ | - | ☑ | - | ☑ | - | - | - | `TestDevSeedService_SeedWorkspaceIsIdempotentForUser` | 一部 item 欠損時の再補完。 |

### 文字列 / ID

| 対象値 / 条件 | empty | whitespace | valid existing | valid missing | malformed | other scope | deleted / inaccessible | max length + 1 / huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| userID | - | - | ☑ | - | - | - | - | - | `TestDevSeedService_SeedWorkspaceIsIdempotentForUser` | empty userID、別 user。 |
