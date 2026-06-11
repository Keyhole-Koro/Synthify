# テストマトリクス: `tree_test.go`

このマトリクスは、`tree_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestTreeService_GetTree_ReturnsItems` | `GetTree` | 正常系 | tree fixture を持つ owner workspace を読む。 | items が返る。 | 状態変化なし。 | `NoError(err)`, `NotEmpty(items)` | member が tree を読めること。 | item 順序、root 種別、全 field。 | root/item fields と ordering の検証。 | PARTIAL |
| [ ] | `TestTreeService_GetTree_EmptyWorkspaceID_ReturnsError` | `GetTree` | 入力 validation | workspaceID 空で読む。 | error を返す。 | 状態変化なし。 | `Error(err)` | 必須 workspaceID の validation。 | error type、userID 空との組み合わせ。 | `ErrInvalidArgument` など具体 error の確認。 | PARTIAL |
| [ ] | `TestTreeService_GetTree_OtherUser_ReturnsForbidden` | `GetTree` | 認可 | stranger が owner workspace の tree を読む。 | `ErrForbidden` を返す。 | 状態変化なし。 | `errors.Is(ErrForbidden)` | 非 member の tree read 拒否。 | share token、revoked member。 | removed member の read 拒否。 | OK |
| [ ] | `TestTreeService_GetSubtree_ItemInWorkspace_ReturnsItems` | `GetSubtree` | 正常系 | workspace 内 root item の subtree を読む。 | items が返り、先頭 item の workspaceID が一致する。 | 状態変化なし。 | `NoError(err)`, `NotEmpty(items)`, `Equal(workspaceID)` | item 所属 workspace が一致する subtree read。 | depth、子孫件数、ordering。 | depth 0 / depth limit の境界。 | PARTIAL |
| [ ] | `TestTreeService_GetSubtree_ItemInOtherWorkspace_ReturnsForbidden` | `GetSubtree` | workspace 境界 | item の所属 workspace と request workspaceID が異なる。 | `ErrForbidden` を返す。 | 状態変化なし。 | `errors.Is(ErrForbidden)` | item 存在を漏らさず workspace 境界を守ること。 | user が両 workspace に member の場合。 | 両 workspace access ありでも item 所属不一致を拒否。 | PARTIAL |
| [ ] | `TestTreeService_GetSubtree_MissingArgs_ReturnsError` | `GetSubtree` | 入力 validation | workspaceID 空、または itemID 空で呼ぶ。 | error を返す。 | 状態変化なし。 | `Error(err)` | 必須引数 validation。 | userID 空、depth 不正。 | depth 0 / negative depth。 | PARTIAL |
| [ ] | `TestTreeService_FindPaths_FindsTreeFromWorkspace` | `FindPaths` | 正常系 | workspace 内の2 item 間 path を探す。 | items が返る。 | 状態変化なし。 | `NoError(err)`, `NotEmpty(items)` | workspace root から path search できること。 | path 内容、not found、depth/limit。 | path の start/end と limit 境界。 | PARTIAL |
| [ ] | `TestTreeService_FindPaths_MissingArgs_ReturnsError` | `FindPaths` | 入力 validation | workspaceID / from / to のいずれかを空にする。 | error を返す。 | 状態変化なし。 | `Error(err)` | 必須引数 validation。 | depth/limit 不正。 | negative depth / zero limit。 | PARTIAL |
| [ ] | `TestTreeService_GetTree_ValidShareToken_AllowsRead` | `GetTree` | share token | userID 空、context に有効 share token を載せる。 | tree read が成功する。 | 状態変化なし。 | `NoError(err)`, `NotEmpty(items)` | 公開リンク token で無認証 read できること。 | role が editor の token、expired token。 | editor token の read 許可、期限付き token の read。 | PARTIAL |
| [ ] | `TestTreeService_GetTree_ShareTokenWrongWorkspace_ReturnsForbidden` | `GetTree` | share token 境界 | token が別 workspace を指している。 | `ErrForbidden` を返す。 | 状態変化なし。 | `ErrorIs(ErrForbidden)` | token workspace 不一致を拒否すること。 | token なし user ありの場合の優先順位。 | user auth と token が矛盾した場合。 | PARTIAL |
| [ ] | `TestTreeService_GetTree_NoUserNoToken_ReturnsForbidden` | `GetTree` | 認証なし | userID 空、token なしで読む。 | `ErrForbidden` を返す。 | 状態変化なし。 | `ErrorIs(ErrForbidden)` | anonymous + no token を拒否すること。 | empty workspaceID との優先順位。 | empty workspaceID + no auth の error mapping。 | OK |
| [ ] | `TestTreeService_GetTree_RevokedShareToken_ReturnsForbidden` | `GetTree` | share token revoke | token 作成後に revoke して読む。 | `ErrForbidden` を返す。 | 状態変化なし。 | `ErrorIs(ErrForbidden)` | revoked token を拒否すること。 | expired token は sharelink 側中心。 | expired token で tree read 拒否。 | PARTIAL |

## 観点別チェックグリッド

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

| テストケース | 正常系 | 異常系 | 入力 validation | 認可 | share token | workspace 境界 | depth/limit | 状態変化 | mock repository |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestTreeService_GetTree_ReturnsItems` | ☑ | - | - | ☑ | - | - | - | - | ☑ |
| `TestTreeService_GetTree_EmptyWorkspaceID_ReturnsError` | - | ☑ | ☑ | - | - | - | - | - | ☑ |
| `TestTreeService_GetTree_OtherUser_ReturnsForbidden` | - | ☑ | - | ☑ | - | - | - | - | ☑ |
| `TestTreeService_GetSubtree_ItemInWorkspace_ReturnsItems` | ☑ | - | - | ☑ | - | ☑ | ◐ | - | ☑ |
| `TestTreeService_GetSubtree_ItemInOtherWorkspace_ReturnsForbidden` | - | ☑ | - | ☑ | - | ☑ | ◐ | - | ☑ |
| `TestTreeService_GetSubtree_MissingArgs_ReturnsError` | - | ☑ | ☑ | - | - | - | ◐ | - | ☑ |
| `TestTreeService_FindPaths_FindsTreeFromWorkspace` | ☑ | - | - | ☑ | - | ◐ | ◐ | - | ☑ |
| `TestTreeService_FindPaths_MissingArgs_ReturnsError` | - | ☑ | ☑ | - | - | - | ◐ | - | ☑ |
| `TestTreeService_GetTree_ValidShareToken_AllowsRead` | ☑ | - | - | ☑ | ☑ | ☑ | - | - | ☑ |
| `TestTreeService_GetTree_ShareTokenWrongWorkspace_ReturnsForbidden` | - | ☑ | - | ☑ | ☑ | ☑ | - | - | ☑ |
| `TestTreeService_GetTree_NoUserNoToken_ReturnsForbidden` | - | ☑ | - | ☑ | ☑ | - | - | - | ☑ |
| `TestTreeService_GetTree_RevokedShareToken_ReturnsForbidden` | - | ☑ | - | ☑ | ☑ | - | - | - | ☑ |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| tree 内容 | items が返ることを中心に確認している。 | ordering、root kind、path 内容、depth/limit 境界。 |
| share token | valid/wrong workspace/no token/revoked は確認済み。 | expired token、auth user と token が矛盾した場合。 |
| workspace 境界 | request workspaceID と item 所属不一致は確認済み。 | caller が両 workspace にアクセスできる場合の所属チェック。 |

## 境界値チェックマトリクス

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 数値 / count / size

| 対象値 / 条件 | `0` | `1` | typical | `max - 1` | `max` | `max + 1` | negative | huge / overflow | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| subtree depth | - | - | ☑ | - | - | - | - | - | `TestTreeService_GetSubtree_ItemInWorkspace_ReturnsItems` | depth 0、depth 1、negative depth、巨大 depth。 |
| path limit | - | - | ☑ | - | - | - | - | - | `TestTreeService_FindPaths_FindsTreeFromWorkspace` | limit 0、limit 1、max limit、max + 1。 |

### 文字列 / ID

| 対象値 / 条件 | empty | whitespace | valid existing | valid missing | malformed | other scope | deleted / inaccessible | max length + 1 / huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| workspaceID | ☑ | - | ☑ | ◐ | - | ☑ | - | - | `TestTreeService_GetTree_ReturnsItems`, `TestTreeService_GetTree_EmptyWorkspaceID_ReturnsError`, `TestTreeService_GetTree_OtherUser_ReturnsForbidden` | malformed ID、unknown workspace と empty の error type 差。 |
| itemID | ☑ | - | ☑ | - | - | ☑ | - | - | `TestTreeService_GetSubtree_ItemInWorkspace_ReturnsItems`, `TestTreeService_GetSubtree_ItemInOtherWorkspace_ReturnsForbidden`, `TestTreeService_GetSubtree_MissingArgs_ReturnsError` | unknown item、malformed itemID。 |
| path endpoints | ☑ | - | ☑ | - | - | - | - | - | `TestTreeService_FindPaths_FindsTreeFromWorkspace`, `TestTreeService_FindPaths_MissingArgs_ReturnsError` | same from/to、not found path。 |
| share token | ☑ | - | ☑ | - | ☑ | ☑ | ☑ | - | `TestTreeService_GetTree_ValidShareToken_AllowsRead`, `TestTreeService_GetTree_ShareTokenWrongWorkspace_ReturnsForbidden`, `TestTreeService_GetTree_NoUserNoToken_ReturnsForbidden`, `TestTreeService_GetTree_RevokedShareToken_ReturnsForbidden` | expired token、auth user と token の workspace 不一致。 |
