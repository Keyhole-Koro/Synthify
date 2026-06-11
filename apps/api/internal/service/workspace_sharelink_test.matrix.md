# テストマトリクス: `workspace_sharelink_test.go`

このマトリクスは、`workspace_sharelink_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestCreateShareLink_Owner_GeneratesToken` | `CreateShareLink` | 正常系 | owner が viewer link を作成する。 | token が生成され、workspace/role/createdBy が一致する。 | share link が永続化される。 | `NotEmpty(Token)`, fields equality | owner による share link 作成。 | expiresAt、token format。 | token length / prefix / expiration 保存。 | PARTIAL |
| [ ] | `TestCreateShareLink_DefaultsToViewer` | `CreateShareLink` | default | role 空で link を作成する。 | role が viewer になる。 | viewer link が永続化される。 | `Equal(Viewer)` | role default。 | 空 role 以外の invalid role。 | unknown role の validation。 | OK |
| [ ] | `TestCreateShareLink_OwnerRole_ReturnsErrInvalidArgument` | `CreateShareLink` | role validation | owner role の link を作成しようとする。 | `ErrInvalidArgument` を返す。 | link を作らない。 | `ErrorIs(ErrInvalidArgument)` | owner role を link に付与できないこと。 | editor role の作成可否。 | editor link 作成と resolve。 | OK |
| [ ] | `TestCreateShareLink_NonOwner_ReturnsErrForbidden` | `CreateShareLink` | 認可 | editor が link を作成しようとする。 | `ErrForbidden` を返す。 | link を作らない。 | `ErrorIs(ErrForbidden)` | owner 以外は link を作れないこと。 | viewer/stranger。 | viewer と non-member の拒否。 | PARTIAL |
| [ ] | `TestCreateShareLink_TokensAreUnique` | `CreateShareLink` | token uniqueness | owner が2つの link を作成する。 | token が異なる。 | 2つの link が作られる。 | `NotEqual(a.Token, b.Token)` | token uniqueness。 | 大量生成時の衝突、DB unique constraint。 | 連続生成 N 件の uniqueness。 | PARTIAL |
| [ ] | `TestResolveShareLink_Valid_ReturnsWorkspaceAndRole` | `ResolveShareLink` | 正常系 | 作成済み token を resolve する。 | workspace と role が返る。 | 状態変化なし。 | `NoError(err)`, workspaceID, role | 有効 token の解決。 | expiresAt future、editor role。 | future expiry の valid resolve。 | PARTIAL |
| [ ] | `TestResolveShareLink_Unknown_ReturnsErrNotFound` | `ResolveShareLink` | missing token | 存在しない token を resolve する。 | `ErrNotFound` を返す。 | 状態変化なし。 | `ErrorIs(ErrNotFound)` | unknown token の存在秘匿。 | 空 token。 | empty token の error mapping。 | PARTIAL |
| [ ] | `TestResolveShareLink_Revoked_ReturnsErrNotFound` | `ResolveShareLink`, `RevokeShareLink` | revoke | token を作成して revoke してから resolve する。 | `ErrNotFound` を返す。 | token が revoked になる。 | `ErrorIs(ErrNotFound)` | revoked token は resolve 不可。 | 二重 revoke、revokedAt の値。 | revoke 後 list 表示、二重 revoke。 | PARTIAL |
| [ ] | `TestResolveShareLink_Expired_ReturnsErrNotFound` | `ResolveShareLink` | expiry | 過去 expiresAt の link を作成して resolve する。 | `ErrNotFound` を返す。 | 状態変化なし。 | `ErrorIs(ErrNotFound)` | expired token は resolve 不可。 | 期限ちょうど、未来期限。 | expiresAt == now の境界。 | PARTIAL |
| [ ] | `TestRevokeShareLink_NonOwner_ReturnsErrForbidden` | `RevokeShareLink` | 認可 | editor が owner の link を revoke しようとする。 | `ErrForbidden` を返す。 | link は revoke されない。 | `ErrorIs(ErrForbidden)` | owner 以外は revoke できないこと。 | stranger、unknown token。 | failed revoke 後に resolve できること。 | PARTIAL |
| [ ] | `TestListShareLinks_NonOwner_ReturnsErrForbidden` | `ListShareLinks` | 認可 | stranger が link list を見る。 | `ErrForbidden` を返す。 | 状態変化なし。 | `ErrorIs(ErrForbidden)` | owner 以外は list できないこと。 | owner の list 成功、editor の拒否。 | owner list の件数と revoked 表示。 | PARTIAL |

## 観点別チェックグリッド

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

| テストケース | 正常系 | 異常系 | 認可 | role validation | token uniqueness | expiry | revoke | list | 状態変化 | 永続化副作用 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestCreateShareLink_Owner_GeneratesToken` | ☑ | - | ☑ | ☑ | ◐ | - | - | - | ☑ | ☑ |
| `TestCreateShareLink_DefaultsToViewer` | ☑ | - | ☑ | ☑ | - | - | - | - | ☑ | ☑ |
| `TestCreateShareLink_OwnerRole_ReturnsErrInvalidArgument` | - | ☑ | - | ☑ | - | - | - | - | ◐ | ◐ |
| `TestCreateShareLink_NonOwner_ReturnsErrForbidden` | - | ☑ | ☑ | - | - | - | - | - | ◐ | ◐ |
| `TestCreateShareLink_TokensAreUnique` | ☑ | - | ☑ | - | ☑ | - | - | - | ☑ | ☑ |
| `TestResolveShareLink_Valid_ReturnsWorkspaceAndRole` | ☑ | - | - | ☑ | - | ◐ | - | - | - | - |
| `TestResolveShareLink_Unknown_ReturnsErrNotFound` | - | ☑ | - | - | - | - | - | - | - | - |
| `TestResolveShareLink_Revoked_ReturnsErrNotFound` | - | ☑ | - | - | - | - | ☑ | - | ☑ | ☑ |
| `TestResolveShareLink_Expired_ReturnsErrNotFound` | - | ☑ | - | - | - | ☑ | - | - | - | - |
| `TestRevokeShareLink_NonOwner_ReturnsErrForbidden` | - | ☑ | ☑ | - | - | - | ☑ | - | ◐ | ◐ |
| `TestListShareLinks_NonOwner_ReturnsErrForbidden` | - | ☑ | ☑ | - | - | - | - | ☑ | - | - |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| list | non-owner 拒否のみ確認している。 | owner list 成功、revoked/expired link の扱い。 |
| revoke | non-owner 拒否と revoked resolve は確認済み。 | owner revoke 成功、failed revoke 後に link が生きていること。 |
| expiry | 過去期限は確認済み。 | future expiry、expiresAt == now の境界。 |
| role | viewer default と owner role 拒否は確認済み。 | editor link 作成/resolve、unknown role。 |

## 境界値チェックマトリクス

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 数値 / count / size

| 対象値 / 条件 | `0` | `1` | typical | `max - 1` | `max` | `max + 1` | negative | huge / overflow | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| token generation count | - | ☑ | ☑ | - | - | - | - | - | `TestCreateShareLink_TokensAreUnique` | N件連続生成、DB unique constraint 衝突。 |
| list result count | - | - | - | - | - | - | - | - | `TestListShareLinks_NonOwner_ReturnsErrForbidden` | owner list 0件/1件/複数件。 |

### 文字列 / ID

| 対象値 / 条件 | empty | whitespace | valid existing | valid missing | malformed | other scope | deleted / inaccessible | max length + 1 / huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| token | - | - | ☑ | ☑ | - | ☑ | ☑ | - | `TestResolveShareLink_Valid_ReturnsWorkspaceAndRole`, `TestResolveShareLink_Unknown_ReturnsErrNotFound`, `TestResolveShareLink_Revoked_ReturnsErrNotFound`, `TestCreateShareLink_TokensAreUnique` | empty token、malformed token、長すぎる token。 |

### enum / role / status

| 対象値 / 条件 | empty / default | allowed value | disallowed value | unknown value | transition before | transition after | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| share role | ☑ | ☑ | ☑ | - | - | - | `TestCreateShareLink_DefaultsToViewer`, `TestCreateShareLink_OwnerRole_ReturnsErrInvalidArgument`, `TestResolveShareLink_Valid_ReturnsWorkspaceAndRole` | editor role、unknown role。 |
| revoke state | - | ☑ | - | - | ☑ | ☑ | `TestResolveShareLink_Revoked_ReturnsErrNotFound`, `TestRevokeShareLink_NonOwner_ReturnsErrForbidden` | owner revoke 成功、二重 revoke、failed revoke 後の link 生存。 |

### 日時 / expiry

| 対象値 / 条件 | missing | past | just before | exactly at | just after | future | invalid format | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| expiresAt | ☑ | ☑ | - | - | - | - | - | `TestResolveShareLink_Expired_ReturnsErrNotFound`, `TestResolveShareLink_Valid_ReturnsWorkspaceAndRole` | future expiry、expiresAt == now、invalid RFC3339。 |
