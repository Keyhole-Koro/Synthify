# テストマトリクス: `workspace_members_test.go`

このマトリクスは、`workspace_members_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

> **読み方の注意**: 下の「テストケース表」は *既存テストが何を確認しているか* を写したものなので、
> **テストが1件も無いメソッド／分岐はこの表に現れない**。
> 「インターフェース網羅チェック」「依存エラー軸」「未テスト分岐 (GAP)」を併読すること。
> カバレッジ数値は `go test -coverprofile` の実測 (2026-06-12)。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

## インターフェース網羅チェック (member 管理)

| メソッド | 専用テスト | coverage | 状態 | 未テストの主分岐 |
| --- | --- | --- | --- | --- |
| `ListMembers` | ✅ 1件 (非member拒否) | 66.7% | PARTIAL | **owner/editor/viewer の list 成功が未確認**、`ListWorkspaceMembers` repo error。 |
| `InviteMember` | ✅ 5件 | 90.0% | OK | `UpsertWorkspaceMember` repo error、重複招待での role 更新。 |
| `UpdateMemberRole` | ✅ 2件 | 70.0% | PARTIAL | invalid/unknown role、unknown target、`GetUser`/`Upsert` repo error。 |
| `RemoveMember` | ✅ 2件 | 100% | OK | — |
| `requireManageMembers` (helper) | ◐ 上記経由 | 83.3% | PARTIAL | `GetWorkspaceRole` repo error。 |

→ `ListMembers` の **成功経路 (member が list を読める) が一度も通っていない**。
非member拒否しか無いので「owner/editor/viewer は list 可」を1件足すべき。

## 依存エラー軸 (dependency returns error)

☑=テスト有 / ◐=間接 / ❌=未テスト。

| メソッド | workspaces repo err | users repo err |
| --- | --- | --- |
| `ListMembers` | ❌ IsWorkspaceAccessible / ListWorkspaceMembers | - |
| `InviteMember` | ❌ GetWorkspaceRole / UpsertWorkspaceMember | ☑ GetUserByEmail (未登録=NotFound) |
| `UpdateMemberRole` | ❌ GetWorkspaceRole / UpsertWorkspaceMember | ❌ GetUser |
| `RemoveMember` | ◐ GetWorkspaceRole / ❌ RemoveWorkspaceMember | - |

→ `InviteMember` の user lookup 失敗のみ確認済み。workspaces repo の書き込み失敗伝播は全面未テスト。

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestInviteMember_Owner_AddsRegisteredUser` | `InviteMember` | 正常系 | owner が登録済み email を editor として招待する。 | member が返り、user/email/role/invitedBy が一致する。 | workspace member が追加される。 | member fields | owner による登録済み user 招待。 | 重複招待、viewer 招待。 | 同じ user の再招待時の role 更新/維持。 | PARTIAL |
| [ ] | `TestInviteMember_UnregisteredEmail_ReturnsErrNotFound` | `InviteMember` | 入力 / lookup | 未登録 email を招待する。 | `ErrNotFound` を返す。 | member を追加しない。 | `ErrorIs(ErrNotFound)` | 未登録 user を招待できないこと。 | email 大文字小文字、空 email。 | email normalization。 | PARTIAL |
| [ ] | `TestInviteMember_OwnerRole_ReturnsErrInvalidArgument` | `InviteMember` | role validation | owner role を明示付与しようとする。 | `ErrInvalidArgument` を返す。 | member を追加しない。 | `ErrorIs(ErrInvalidArgument)` | owner role は招待で付与できないこと。 | 空 role default、未知 role。 | invalid role 文字列。 | OK |
| [ ] | `TestInviteMember_NonOwnerCaller_ReturnsErrForbidden` | `InviteMember` | 認可 | editor が別 user を招待しようとする。 | `ErrForbidden` を返す。 | member を追加しない。 | `ErrorIs(ErrForbidden)` | owner 以外は招待できないこと。 | viewer caller。 | viewer も invite できないこと。 | PARTIAL |
| [ ] | `TestInviteMember_Stranger_ReturnsErrForbidden` | `InviteMember` | 認可 | 非 member が招待しようとする。 | `ErrForbidden` を返す。 | member を追加しない。 | `ErrorIs(ErrForbidden)` | stranger は招待できないこと。 | unknown workspace。 | unknown workspace の存在秘匿。 | PARTIAL |
| [ ] | `TestGetWorkspace_InvitedMember_HasAccess` | `GetWorkspace`, `InviteMember` | access propagation | 招待前後で invitee の workspace access を確認する。 | 招待前は Forbidden、招待後は取得成功。 | invite により member access が付与される。 | before `ErrForbidden`, after `NoError`, workspaceID | 招待が read access に反映されること。 | editor/viewer role 差、list members。 | role 別 GetWorkspace の同値性。 | OK |
| [ ] | `TestUpdateMemberRole_Owner_UpdatesRole` | `UpdateMemberRole` | role update | owner が invitee の viewer を editor に変更する。 | member role と persisted role が editor になる。 | workspace member role が更新される。 | `Equal(Editor)`, `GetWorkspaceRole` | owner による role 更新と永続化。 | owner role への更新、self update。 | owner role 更新拒否、存在しない member。 | PARTIAL |
| [ ] | `TestUpdateMemberRole_NonOwner_ReturnsErrForbidden` | `UpdateMemberRole` | 認可 | editor が role 更新しようとする。 | `ErrForbidden` を返す。 | role を変えない。 | `ErrorIs(ErrForbidden)` | owner 以外は role 更新不可。 | viewer caller、stranger caller。 | failed update 後の role 不変 assert。 | PARTIAL |
| [ ] | `TestRemoveMember_Owner_RemovesMember` | `RemoveMember` | member removal | owner が viewer member を削除する。 | 削除成功し、access が消える。 | workspace member が削除される。 | `NoError(err)`, `False(accessible)` | owner による member 削除。 | owner 自身の削除、再削除。 | owner removal 拒否、二重 remove。 | PARTIAL |
| [ ] | `TestRemoveMember_NonOwner_ReturnsErrForbidden` | `RemoveMember` | 認可 | editor が自分を remove しようとする。 | `ErrForbidden` を返す。 | member を削除しない。 | `ErrorIs(ErrForbidden)` | owner 以外は remove 不可。 | self-leave 機能の有無。 | failed remove 後 access 維持。 | PARTIAL |
| [ ] | `TestRemoveMember_Unknown_ReturnsErrNotFound` | `RemoveMember` | missing target | owner が存在しない member を削除する。 | `ErrNotFound` を返す。 | 状態変化なし。 | `ErrorIs(ErrNotFound)` | absent member の error mapping。 | unknown workspace。 | unknown workspace では Forbidden/NotFound どちらか。 | OK |
| [ ] | `TestListMembers_NonMember_ReturnsErrForbidden` | `ListMembers` | 認可 | stranger が member list を見る。 | `ErrForbidden` を返す。 | 状態変化なし。 | `ErrorIs(ErrForbidden)` | 非 member は member list を読めないこと。 | viewer/editor/owner の list 許可。 | viewer/editor が list できるか。 | PARTIAL |
| [ ] | `TestListWorkspaces_InvitedMember_SeesSharedWorkspace` | `ListWorkspaces` | shared listing | invitee の workspace list を招待前後で見る。 | 招待前は空、招待後は shared workspace が1件出る。 | membership が list に反映される。 | `Empty(before)`, `Len(after, 1)`, `Equal(wsID)` | 招待済み member の一覧導線。 | 複数 workspace、role 情報、削除後。 | remove 後に list から消えること。 | PARTIAL |

## 観点別チェックグリッド

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

| テストケース | 正常系 | 異常系 | 認可 | role validation | access propagation | 状態変化 | 永続化副作用 | list 導線 | mock repository |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestInviteMember_Owner_AddsRegisteredUser` | ☑ | - | ☑ | ◐ | ◐ | ☑ | ☑ | - | ☑ |
| `TestInviteMember_UnregisteredEmail_ReturnsErrNotFound` | - | ☑ | ☑ | - | - | ◐ | ◐ | - | ☑ |
| `TestInviteMember_OwnerRole_ReturnsErrInvalidArgument` | - | ☑ | - | ☑ | - | ◐ | ◐ | - | ☑ |
| `TestInviteMember_NonOwnerCaller_ReturnsErrForbidden` | - | ☑ | ☑ | - | - | ◐ | ◐ | - | ☑ |
| `TestInviteMember_Stranger_ReturnsErrForbidden` | - | ☑ | ☑ | - | - | ◐ | ◐ | - | ☑ |
| `TestGetWorkspace_InvitedMember_HasAccess` | ☑ | ☑ | ☑ | ◐ | ☑ | ☑ | ☑ | - | ☑ |
| `TestUpdateMemberRole_Owner_UpdatesRole` | ☑ | - | ☑ | ☑ | ☑ | ☑ | ☑ | - | ☑ |
| `TestUpdateMemberRole_NonOwner_ReturnsErrForbidden` | - | ☑ | ☑ | - | - | ◐ | ◐ | - | ☑ |
| `TestRemoveMember_Owner_RemovesMember` | ☑ | - | ☑ | - | ☑ | ☑ | ☑ | - | ☑ |
| `TestRemoveMember_NonOwner_ReturnsErrForbidden` | - | ☑ | ☑ | - | - | ◐ | ◐ | - | ☑ |
| `TestRemoveMember_Unknown_ReturnsErrNotFound` | - | ☑ | ☑ | - | - | - | - | - | ☑ |
| `TestListMembers_NonMember_ReturnsErrForbidden` | - | ☑ | ☑ | - | - | - | - | ☑ | ☑ |
| `TestListWorkspaces_InvitedMember_SeesSharedWorkspace` | ☑ | - | ☑ | - | ☑ | ☑ | ☑ | ☑ | ☑ |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| role validation | owner role 拒否と editor update は確認済み。 | unknown role、empty role default、owner role への update 拒否。 |
| failed mutation | error は確認している。 | failed invite/update/remove 後に状態が変わっていないこと。 |
| list 導線 | invited workspace が list に出ることは確認済み。 | remove 後に list から消えること、複数 workspace の順序。 |

## 境界値チェックマトリクス

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 数値 / count / size

| 対象値 / 条件 | `0` | `1` | typical | `max - 1` | `max` | `max + 1` | negative | huge / overflow | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| workspace list count | ☑ | ☑ | - | - | - | - | - | - | `TestListWorkspaces_InvitedMember_SeesSharedWorkspace` | 複数 workspace、remove 後 0 件に戻ること。 |

### 文字列 / ID

| 対象値 / 条件 | empty | whitespace | valid existing | valid missing | malformed | other scope | deleted / inaccessible | max length + 1 / huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| invite email | - | - | ☑ | ☑ | - | - | - | - | `TestInviteMember_Owner_AddsRegisteredUser`, `TestInviteMember_UnregisteredEmail_ReturnsErrNotFound` | empty email、大文字小文字、whitespace、invalid email format。 |
| target userID | - | - | ☑ | ☑ | - | - | - | - | `TestRemoveMember_Unknown_ReturnsErrNotFound`, `TestRemoveMember_Owner_RemovesMember` | update unknown member、malformed userID、invite duplicate member。 |

### enum / role / status

| 対象値 / 条件 | empty / default | allowed value | disallowed value | unknown value | transition before | transition after | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| invited / updated role | - | ☑ | ☑ | - | ☑ | ☑ | `TestInviteMember_OwnerRole_ReturnsErrInvalidArgument`, `TestUpdateMemberRole_Owner_UpdatesRole` | empty role、unknown role、owner role への update。 |
| caller role | - | ☑ | ☑ | ◐ | - | - | `TestInviteMember_NonOwnerCaller_ReturnsErrForbidden`, `TestInviteMember_Stranger_ReturnsErrForbidden`, `TestRemoveMember_NonOwner_ReturnsErrForbidden` | viewer caller、削除済み member、owner self-removal。 |
