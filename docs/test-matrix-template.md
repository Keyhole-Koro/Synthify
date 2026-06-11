# テストマトリクステンプレート

このテンプレートは、1つのテストファイルに対して `*.matrix.md` を作るためのひな形です。

推奨ファイル名:

```txt
<test-file-name>.matrix.md
```

例:

```txt
document_test.go
document_test.matrix.md

workspaceSnapshotCache.test.ts
workspaceSnapshotCache.test.matrix.md
```

## テストマトリクス: `<test-file-name>`

このマトリクスは、`<test-file-name>` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

ステータス:

| ステータス | 意味 |
| --- | --- |
| OK | 主要な挙動はこのテストファイルで担保している。 |
| PARTIAL | 有用な挙動は担保しているが、重要な境界値や統合経路はこのファイルの外に残っている。 |
| GAP | 必要な確認観点だが、現時点ではテストで担保されていない。 |

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `<TestCaseName>` | `<関数 / hook / component / endpoint>` | `<観点>` | `<テスト前提と入力>` | `<期待される戻り値 / error / 表示>` | `<DB / state / cache / job / event の変化>` | `<主要な assertion>` | `<このテストで担保する仕様>` | `<このテストでは見ない境界や統合経路>` | `<次に追加するとよいテスト>` | `<OK / PARTIAL / GAP>` |
| [ ] | `<TestCaseName>` | `<関数 / hook / component / endpoint>` | `<観点>` | `<テスト前提と入力>` | `<期待される戻り値 / error / 表示>` | `<DB / state / cache / job / event の変化>` | `<主要な assertion>` | `<このテストで担保する仕様>` | `<このテストでは見ない境界や統合経路>` | `<次に追加するとよいテスト>` | `<OK / PARTIAL / GAP>` |

## 観点別チェックグリッド

この表は、各テストケースがどの確認観点を担保しているかを横断的に見るためのものです。

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

観点カラムは、対象のテストファイルに合わせて増減してください。

| テストケース | 正常系 | 異常系 | 境界値 | 認可 | 入力 validation | 状態変化 | 永続化副作用 | 外部依存 mock | idempotency / retry | UI / 表示 | 非同期 / race |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `<TestCaseName>` | - | - | - | - | - | - | - | - | - | - | - |
| `<TestCaseName>` | - | - | - | - | - | - | - | - | - | - | - |

## 観点別の穴

この表は、チェックグリッドから見えた不足をまとめるためのものです。

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| `<観点>` | `<確認済みの範囲>` | `<不足している境界値 / 異常系 / 統合経路>` |
| `<観点>` | `<確認済みの範囲>` | `<不足している境界値 / 異常系 / 統合経路>` |

## 境界値チェックマトリクス

この表は、値の境界を網羅的に見られているかを確認するためのものです。対象値の型に合わせて、下の表から必要なものだけ使ってください。

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 数値 / count / size

| 対象値 / 条件 | `0` | `1` | typical | `max - 1` | `max` | `max + 1` | negative | huge / overflow | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `<size / count / limit>` | - | - | - | - | - | - | - | - | `<TestCaseName>` | `<足りない境界>` |

### 文字列 / ID

| 対象値 / 条件 | empty | whitespace | valid existing | valid missing | malformed | other scope | deleted / inaccessible | max length + 1 / huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `<id / name / token / email>` | - | - | - | - | - | - | - | - | `<TestCaseName>` | `<足りない境界>` |

### enum / role / status

| 対象値 / 条件 | empty / default | allowed value | disallowed value | unknown value | transition before | transition after | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `<role / status / plan>` | - | - | - | - | - | - | `<TestCaseName>` | `<足りない境界>` |

### 日時 / expiry

| 対象値 / 条件 | missing | past | just before | exactly at | just after | future | invalid format | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `<expiresAt / timestamp>` | - | - | - | - | - | - | - | `<TestCaseName>` | `<足りない境界>` |

## 観点カラムの例

バックエンド service / repository:

```txt
正常系 / 異常系 / 境界値 / 認可 / quota / budget / metadata 検証 /
状態変化 / job 作成・抑止 / dispatcher / idempotency・retry /
外部依存 mock / 永続化副作用
```

API handler / middleware:

```txt
正常系 / 異常系 / 認証 / 認可 / request validation /
status code / response body / error mapping / side effect / logging
```

フロントエンド hook / util:

```txt
正常系 / 異常系 / 境界値 / cache / state transition /
非同期 / timer / error handling / cleanup / persistence
```

フロントエンド component:

```txt
render / interaction / loading / empty state / error state /
accessibility / responsive / state transition / side effect
```
