# テストマトリクス: `item_test.go`

このマトリクスは、`item_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

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

## インターフェース網羅チェック (`ItemUsecase`)

| メソッド | 専用テスト | coverage | 状態 | 未テストの主分岐 |
| --- | --- | --- | --- | --- |
| `GetItem` | ✅ 2件 | 75.0% | PARTIAL | `GetItem` repo error、authorizeWorkspace repo error。 |
| `CreateItem` | ✅ 4件 | 100% | OK | `CreateItem` repo error。 |
| `ApproveAlias` | ❌ **なし** | **0.0%** | GAP | 全分岐 — write 認可拒否 (viewer/non-member) / 正常承認 / `ApproveAlias` repo error。 |
| `RejectAlias` | ❌ **なし** | **0.0%** | GAP | 全分岐 — write 認可拒否 / 正常却下 / `RejectAlias` repo error。 |

→ **alias の承認/却下 (`ApproveAlias` / `RejectAlias`) がまるごと未テスト。** どちらも `authorizeWrite` →
`repo.<Op>` の薄いラッパなので、`CreateItem` の viewer/non-member 拒否テストを移植 + 正常系1件で埋まる。
alias は item の重複統合に関わるため、認可漏れは canonical/alias の取り違えに直結する。

## 依存エラー軸 (dependency returns error)

☑=テスト有 / ◐=間接 / ❌=未テスト。

| メソッド | workspaces (authz) repo err | item repo err |
| --- | --- | --- |
| `GetItem` | ❌ IsWorkspaceAccessible | ❌ GetItem |
| `CreateItem` | ◐ GetWorkspaceRole (Forbidden) | ❌ CreateItem |
| `ApproveAlias` | ❌ GetWorkspaceRole | ❌ ApproveAlias |
| `RejectAlias` | ❌ GetWorkspaceRole | ❌ RejectAlias |

→ 認可拒否 (Forbidden) は確認済みだが、repo の error 伝播は全面未テスト。

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestCreateItem_CreatesItem` | `CreateItem` | 正常系 | owner の workspace に root item を作成する。 | item 作成が成功する。 | workspace 配下に item が永続化される。 | `NoError(err)`, `NotNil(item)` | owner が item を作成できること。 | item fields の完全性、parent/edge の検証。 | title / description / parent ID の保存値を assert する。 | PARTIAL |
| [ ] | `TestCreateItem_NonMember_ReturnsForbidden` | `CreateItem` | 認可 | 非 member が owner workspace に item を作成しようとする。 | `ErrForbidden` を返す。 | item を作成しない。 | `ErrorIs(ErrForbidden)` | workspace 非 member の write 拒否。 | 存在しない workspace との区別、nil/empty userID。 | role なし user と unknown workspace の優先順位。 | PARTIAL |
| [ ] | `TestCreateItem_Viewer_ReturnsForbidden` | `CreateItem` | role 認可 | viewer が item を作成しようとする。 | `ErrForbidden` を返す。 | item を作成しない。 | `ErrorIs(ErrForbidden)` | viewer は write できないこと。 | viewer の read 可否は別テスト。 | viewer 作成失敗後に item 数が増えないこと。 | OK |
| [ ] | `TestCreateItem_Editor_Succeeds` | `CreateItem` | role 認可 | editor が item を作成する。 | item 作成が成功する。 | editor による item が永続化される。 | `NoError(err)`, `NotNil(item)` | editor は write できること。 | created_by attribution、owner workspace quota。 | `CreatedBy` など監査フィールドの検証。 | PARTIAL |
| [ ] | `TestGetItem_Viewer_HasReadAccess` | `GetItem` | read 認可 | owner が item を作成し、viewer が取得する。 | viewer が item を取得できる。 | 状態変化なし。 | `NoError(err)`, `Equal(created.ItemID, got.ItemID)` | viewer は read できること。 | share token 経由 read、削除済み member。 | role なし user / revoked member の read 拒否。 | PARTIAL |
| [ ] | `TestGetItem_OtherWorkspaceID_ReturnsForbidden` | `GetItem` | workspace 境界 | item の実 workspace と異なる workspaceID で取得する。 | `ErrForbidden` を返す。 | 状態変化なし。 | `ErrorIs(ErrForbidden)` | workspaceID 不一致時に存在を漏らさないこと。 | user が両 workspace の member の場合。 | 両 workspace に権限がある user で item 所属チェックを確認する。 | PARTIAL |

## 観点別チェックグリッド

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

| テストケース | 正常系 | 異常系 | 認可 | role 境界 | workspace 境界 | 状態変化 | 永続化副作用 | mock repository |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestCreateItem_CreatesItem` | ☑ | - | ◐ | - | - | ☑ | ☑ | ☑ |
| `TestCreateItem_NonMember_ReturnsForbidden` | - | ☑ | ☑ | - | ◐ | ◐ | ◐ | ☑ |
| `TestCreateItem_Viewer_ReturnsForbidden` | - | ☑ | ☑ | ☑ | - | ◐ | ◐ | ☑ |
| `TestCreateItem_Editor_Succeeds` | ☑ | - | ☑ | ☑ | - | ☑ | ☑ | ☑ |
| `TestGetItem_Viewer_HasReadAccess` | ☑ | - | ☑ | ☑ | - | - | - | ☑ |
| `TestGetItem_OtherWorkspaceID_ReturnsForbidden` | - | ☑ | ☑ | - | ☑ | - | - | ☑ |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| item fields | item の存在だけを主に確認している。 | title / description / parent ID / created_by の保存値。 |
| 認可 | owner/editor write、viewer/non-member deny、viewer read は確認済み。 | role なし user、削除済み member、share token read。 |
| workspace 境界 | 別 workspaceID 指定は Forbidden を確認済み。 | user が両 workspace に権限を持つ場合でも item 所属 workspace を守ること。 |

## 境界値チェックマトリクス

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 文字列 / ID

| 対象値 / 条件 | empty | whitespace | valid existing | valid missing | malformed | other scope | deleted / inaccessible | max length + 1 / huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| item title / description | ☑ | - | - | - | - | - | - | - | `TestCreateItem_CreatesItem`, `TestCreateItem_Editor_Succeeds` | whitespace only、max length、長すぎる文字列、保存値の明示 assert。 |
| parent item ID | ☑ | - | ◐ | - | - | - | - | - | `TestCreateItem_CreatesItem` | 存在しない parent、別 workspace parent、削除済み parent。 |
| workspaceID | - | - | ☑ | ◐ | - | ☑ | - | - | `TestGetItem_OtherWorkspaceID_ReturnsForbidden` | empty workspaceID、malformed workspaceID、両 workspace 権限ありでの所属チェック。 |

### enum / role / status

| 対象値 / 条件 | empty / default | allowed value | disallowed value | unknown value | transition before | transition after | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| workspace role | - | ☑ | ☑ | ◐ | - | - | `TestCreateItem_NonMember_ReturnsForbidden`, `TestCreateItem_Viewer_ReturnsForbidden`, `TestCreateItem_Editor_Succeeds`, `TestGetItem_Viewer_HasReadAccess` | owner 明示、role なし user、削除済み member、share token read。 |
