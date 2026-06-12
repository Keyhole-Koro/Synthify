# テストマトリクス: `workspace_test.go`

このマトリクスは、`workspace_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

> **読み方の注意**: 下の「テストケース表」は *既存テストが何を確認しているか* を写したものなので、
> **テストが1件も無いメソッド／分岐はこの表に現れない**。取りこぼしを防ぐため、
> 「インターフェース網羅チェック」「依存エラー軸」「未テスト分岐 (GAP)」の各節を併読すること。
> カバレッジ数値は `go test -coverprofile ./apps/api/internal/service` の実測 (2026-06-12)。
> `WorkspaceUsecase` のメンバー系は [workspace_members](workspace_members_test.matrix.md)、
> share link 系は [workspace_sharelink](workspace_sharelink_test.matrix.md) を参照。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

## インターフェース網羅チェック (workspace CRUD)

`WorkspaceUsecase` のうち workspace 本体の CRUD を対象に、専用テストの有無と実測カバレッジを突き合わせる。

| メソッド | 専用テスト | coverage | 状態 | 未テストの主分岐 |
| --- | --- | --- | --- | --- |
| `ListWorkspaces` | ◐ members 側で間接 | 100% | OK | repo error。 |
| `GetWorkspace` | ✅ 3件 | 77.8% | PARTIAL | `GetWorkspace` repo error、editor/viewer member の read。 |
| `CreateWorkspace` | ✅ 2件 | 100% | OK | `CreateWorkspace` repo error。 |
| `UpdateWorkspace` | ❌ **なし** | **0.0%** | GAP | 全分岐 — 認可拒否 / 正常更新 / repo error。 |
| `DeleteWorkspace` | ❌ **なし** | **0.0%** | GAP | 全分岐 — 認可拒否 / 正常削除 / repo error。 |

→ **`UpdateWorkspace` / `DeleteWorkspace` がまるごと未テスト。** 既存の `GetWorkspace` 系 (`IsWorkspaceAccessible`→`ErrForbidden`) と同型なので、
non-member 拒否・owner 成功・persisted name/削除確認の3観点を移植するだけで埋まる。

## 依存エラー軸 (dependency returns error)

各メソッドが repo のエラーをどう伝播するか。☑=テスト有 / ◐=間接 / ❌=未テスト。

| メソッド | accounts repo err | workspaces repo err |
| --- | --- | --- |
| `ListWorkspaces` | - | ❌ ListWorkspacesByUser |
| `GetWorkspace` | - | ❌ IsWorkspaceAccessible / GetWorkspace |
| `CreateWorkspace` | ☑ GetAccountByUser (NoAccount) | ❌ CreateWorkspace |
| `UpdateWorkspace` | - | ❌ IsWorkspaceAccessible / UpdateWorkspaceName |
| `DeleteWorkspace` | - | ❌ IsWorkspaceAccessible / DeleteWorkspace |

→ `CreateWorkspace` の account 欠如のみ確認済み。それ以外の repo error 伝播は全面未テスト
（mock store にフォールト注入フックが無いため、failing decorator が前提）。

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestGetWorkspace_NonMember_ReturnsErrForbidden` | `GetWorkspace` | 認可 | stranger が owner workspace を取得する。 | `ErrForbidden` を返す。 | 状態変化なし。 | `ErrorIs(ErrForbidden)` | 非 member の workspace read 拒否。 | share token、削除済み member。 | invited member / revoked member の比較。 | OK |
| [ ] | `TestGetWorkspace_Member_ReturnsWorkspace` | `GetWorkspace` | 正常系 | owner が自分の workspace を取得する。 | workspace を返す。 | 状態変化なし。 | `NoError(err)`, `Equal(wsID, got.WorkspaceID)` | member は workspace を読めること。 | workspace fields 全体、editor/viewer member。 | editor/viewer の `GetWorkspace` 成功。 | PARTIAL |
| [ ] | `TestGetWorkspace_UnknownID_ReturnsErrForbidden` | `GetWorkspace` | 存在秘匿 | 存在しない workspaceID を取得する。 | `ErrForbidden` を返す。 | 状態変化なし。 | `ErrorIs(ErrForbidden)` | unknown workspace でも存在有無を漏らさないこと。 | caller が空の場合。 | empty userID / empty workspaceID の error mapping。 | OK |
| [ ] | `TestCreateWorkspace_NoAccount_ReturnsError` | `CreateWorkspace` | 事前条件 | account がない user で workspace を作成する。 | `ErrNotFound` を返す。 | workspace を作らない。 | `ErrorIs(ErrNotFound)` | workspace 作成には account が必要なこと。 | account 作成 race、user row だけある場合。 | orphan user row での扱い。 | PARTIAL |
| [ ] | `TestCreateWorkspace_Success_CreatesWorkspace` | `CreateWorkspace` | 正常系 | account 作成済み owner が workspace を作成する。 | workspace 作成が成功し、name が一致する。 | workspace と owner membership が作られる。 | `NoError(err)`, `NotNil(ws)`, `Equal(name)` | account 持ち user の workspace 作成。 | root item 作成、quota 初期値、owner role。 | 作成後 `GetWorkspaceRole` が owner であること。 | PARTIAL |

## 観点別チェックグリッド

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

| テストケース | 正常系 | 異常系 | 認可 | 存在秘匿 | 事前条件 | 状態変化 | 永続化副作用 | mock repository |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestGetWorkspace_NonMember_ReturnsErrForbidden` | - | ☑ | ☑ | ◐ | - | - | - | ☑ |
| `TestGetWorkspace_Member_ReturnsWorkspace` | ☑ | - | ☑ | - | - | - | - | ☑ |
| `TestGetWorkspace_UnknownID_ReturnsErrForbidden` | - | ☑ | ☑ | ☑ | - | - | - | ☑ |
| `TestCreateWorkspace_NoAccount_ReturnsError` | - | ☑ | - | - | ☑ | ◐ | ◐ | ☑ |
| `TestCreateWorkspace_Success_CreatesWorkspace` | ☑ | - | - | - | ☑ | ☑ | ☑ | ☑ |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| member role | owner の read は確認済み。 | editor/viewer member の read 成功。 |
| 作成副作用 | workspace name は確認済み。 | owner role、root item、初期 plan / quota の確認。 |
| 入力 validation | account なしは確認済み。 | empty name、長すぎる name、empty userID。 |

## 境界値チェックマトリクス

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 文字列 / ID

| 対象値 / 条件 | empty | whitespace | valid existing | valid missing | malformed | other scope | deleted / inaccessible | max length + 1 / huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| workspaceID | - | - | ☑ | ☑ | - | - | - | - | `TestGetWorkspace_Member_ReturnsWorkspace`, `TestGetWorkspace_UnknownID_ReturnsErrForbidden` | empty workspaceID、malformed workspaceID。 |
| workspace name | - | - | ☑ | - | - | - | - | - | `TestCreateWorkspace_Success_CreatesWorkspace` | empty name、whitespace only、max length、max length + 1。 |
| userID | - | - | ☑ | ☑ | - | - | - | - | `TestCreateWorkspace_NoAccount_ReturnsError`, `TestCreateWorkspace_Success_CreatesWorkspace` | empty userID、malformed userID。 |

### enum / role / status

| 対象値 / 条件 | empty / default | allowed value | disallowed value | unknown value | transition before | transition after | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| member role | - | ☑ | ☑ | ◐ | - | - | `TestGetWorkspace_NonMember_ReturnsErrForbidden`, `TestGetWorkspace_Member_ReturnsWorkspace` | editor/viewer member、削除済み member。 |
