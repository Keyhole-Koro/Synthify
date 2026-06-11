# テストマトリクス: `document_image_url_test.go`

このマトリクスは、`document_image_url_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestIssueImageURL_OwnerGetsSignedURL` | `IssueImageURL` | 正常系 | owner workspace に document file を作り、owner が signed URL を要求する。 | signed URL が返る。issuer には file の workspace/document/filename が渡る。 | 状態変化なし。 | `NoError(err)`, `NotEmpty(URL)`, issuer fields | owner が document image の signed URL を取得できること。 | URL 有効期限、実 GCS signer、content type。 | issuer に渡す workspace/document ID の具体値比較。 | PARTIAL |
| [ ] | `TestIssueImageURL_NonMemberForbidden` | `IssueImageURL` | 認可 | workspace 非 member の intruder が fileID で URL を要求する。 | `ErrForbidden` を返す。 | issuer を呼ばない想定。 | `ErrorIs(ErrForbidden)` | 非 member には image URL を発行しないこと。 | issuer 未呼び出しの明示 assertion、viewer/editor。 | forbidden 時に issuer call count が 0 であること。 | PARTIAL |
| [ ] | `TestIssueImageURL_UnknownFileForbidden` | `IssueImageURL` | 存在秘匿 | 存在しない fileID で URL を要求する。 | `ErrForbidden` を返す。 | issuer を呼ばない想定。 | `ErrorIs(ErrForbidden)` | missing file を NotFound で漏らさないこと。 | user が存在する場合/しない場合の違い。 | unknown file で issuer 未呼び出しを assert。 | PARTIAL |

## 観点別チェックグリッド

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

| テストケース | 正常系 | 異常系 | 認可 | 存在秘匿 | signed URL | issuer 引数 | issuer 抑止 | 外部依存 mock | 状態変化 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestIssueImageURL_OwnerGetsSignedURL` | ☑ | - | ☑ | - | ☑ | ☑ | - | ☑ | - |
| `TestIssueImageURL_NonMemberForbidden` | - | ☑ | ☑ | ◐ | - | - | ◐ | ☑ | - |
| `TestIssueImageURL_UnknownFileForbidden` | - | ☑ | ☑ | ☑ | - | - | ◐ | ☑ | - |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| issuer | 正常系では引数を一部確認している。 | forbidden / unknown file で issuer が呼ばれないこと。 |
| role | owner の成功、non-member の拒否は確認済み。 | viewer/editor の read 許可可否。 |
| 外部依存 | fake issuer で URL を返している。 | 実 GCS signer の integration、URL 有効期限。 |

## 境界値チェックマトリクス

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 文字列 / ID

| 対象値 / 条件 | empty | whitespace | valid existing | valid missing | malformed | other scope | deleted / inaccessible | max length + 1 / huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| fileID | - | - | ☑ | ☑ | - | ◐ | - | - | `TestIssueImageURL_OwnerGetsSignedURL`, `TestIssueImageURL_UnknownFileForbidden`, `TestIssueImageURL_NonMemberForbidden` | empty fileID、malformed fileID、他 workspace file、削除済み file。 |

### enum / role / status

| 対象値 / 条件 | empty / default | allowed value | disallowed value | unknown value | transition before | transition after | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| workspace access | - | ☑ | ☑ | ◐ | - | - | `TestIssueImageURL_OwnerGetsSignedURL`, `TestIssueImageURL_NonMemberForbidden` | viewer/editor の read 許可可否、削除済み member。 |

### 外部依存

| 対象値 / 条件 | not configured | success | returns empty | returns error | called once | not called | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| ImageURLIssuer | - | ☑ | - | - | ☑ | ◐ | `TestIssueImageURL_OwnerGetsSignedURL` | forbidden / unknown file で issuer が呼ばれないこと、issuer error。 |
