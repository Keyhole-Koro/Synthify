# テストマトリクス: `user_test.go`

このマトリクスは、`user_test.go` の各テストケースが何を保証していて、何を意図的にカバーしていないかを確認するための表です。

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

## インターフェース網羅チェック (`UserUsecase`)

| メソッド | 専用テスト | coverage | 状態 | 未テストの主分岐 |
| --- | --- | --- | --- | --- |
| `SignInUser` | ✅ 3件 | 75.0% | PARTIAL | **`userID == ""` の早期 error**、`upsertUserRow` / `ensureAccount` の repo error 伝播。 |

内部 helper の実測: `upsertUserRow` 91.7% (`GetUser` の非NotFound error) / `ensureAccount` 77.8% (`CreateAccount` error) /
`grantSignupCreditIfFirstTime` **50.0%**。

→ `grantSignupCreditIfFirstTime` は **`billing == nil` 経路と「grant が error を返してもログのみで sign-in 成功」経路が未テスト**。
後者は仕様 (credit 失敗で sign-in 全体を落とさない) の中核なので、billing が error を返す fake で
`SignInUser` が成功し IsNewAccount=true を返すことを固めるべき。

## 依存エラー軸 (dependency returns error)

☑=テスト有 / ◐=間接 / ❌=未テスト。

| 経路 | 条件 | 期待 | 状態 |
| --- | --- | --- | --- |
| 入力検証 | `userID == ""` | error | ❌ |
| users repo | `GetUser` 非NotFound error / `UpsertUser` error | 伝播 | ❌ |
| accounts repo | `GetAccountByUser` 非NotFound / `CreateAccount` error | 伝播 | ❌ |
| billing | `GrantFreeSignupCredit` が error | **ログのみ・sign-in 成功** | ❌ |
| billing 未配線 | `billing == nil` | grant skip・sign-in 成功 | ❌ |

→ 正常系3経路 (新規 / 既存 / orphan 復旧) は手厚いが、**異常系・依存エラーが1件も無い**。
特に billing grant 失敗時に sign-in が成功し続けることの確認が抜けている。

| チェック | テストケース | 対象 | 観点 | セットアップ / 入力 | 期待結果 | 副作用 / 状態変化 | 主要 assertion | カバーしていること | カバーしていないこと | 追加候補 | ステータス |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ ] | `TestSignInUser_NewUser_CreatesAccountAndGrantsCredit` | `SignInUser` | 新規 sign-in | 未登録 userID/email/displayName で sign-in する。 | user/account が作成され、signup credit が付与される。 | user row、account row、billing grant が発生する。 | `IsNewAccount`, user fields, account lookup, `grantFreeCalls == 1` | 新規ユーザー初回ログイン処理。 | billing grant 失敗時、email/displayName 空文字。 | `GrantFreeSignupCredit` error の rollback / error handling。 | PARTIAL |
| [ ] | `TestSignInUser_ExistingUser_UpdatesLastLoginAndNoNewCredit` | `SignInUser` | 既存 sign-in | user/account 作成済みで再 sign-in する。 | `LastLoginAt` と displayName を更新し、credit は付与しない。 | user row は更新、account/billing grant は増えない。 | `False(IsNewAccount)`, `CreatedAt` 維持, `grantFreeCalls == 0` | 既存 user の再ログイン idempotency。 | email 変更、account 欠損。 | email 更新ポリシーの明示テスト。 | OK |
| [ ] | `TestSignInUser_OrphanUserRow_RecoversByCreatingAccount` | `SignInUser` | 部分失敗復旧 | user row のみ存在し、account がない状態で sign-in する。 | account を作成し、signup credit を付与する。 | missing account が補完される。 | `True(IsNewAccount)`, `CreatedAt` 維持, account lookup, `grantFreeCalls == 1` | user/account 作成途中失敗からの自己回復。 | billing grant 失敗、同時ログイン race。 | orphan recovery の並行実行。 | PARTIAL |

## 観点別チェックグリッド

| 記号 | 意味 |
| --- | --- |
| ☑ | 主要な assertion として確認している。 |
| ◐ | 間接的に確認している、または一部だけ確認している。 |
| - | このテストケースの対象外。 |

| テストケース | 正常系 | 異常系 | 境界/復旧 | user 永続化 | account 永続化 | billing 副作用 | time 固定 | idempotency | mock billing |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `TestSignInUser_NewUser_CreatesAccountAndGrantsCredit` | ☑ | - | - | ☑ | ☑ | ☑ | ☑ | - | ☑ |
| `TestSignInUser_ExistingUser_UpdatesLastLoginAndNoNewCredit` | ☑ | - | ◐ | ☑ | ◐ | ☑ | ☑ | ☑ | ☑ |
| `TestSignInUser_OrphanUserRow_RecoversByCreatingAccount` | ☑ | - | ☑ | ☑ | ☑ | ☑ | ☑ | ◐ | ☑ |

## 観点別の穴

| 観点 | 現状 | 追加するとよいチェック |
| --- | --- | --- |
| billing error | 成功呼び出し回数のみ確認している。 | credit grant が失敗したときの戻り値と永続化状態。 |
| 入力 validation | 通常の userID/email/displayName のみ確認している。 | empty email、empty userID、displayName 空文字。 |
| 並行性 | 単一呼び出しのみ確認している。 | 初回 sign-in / orphan recovery の同時実行。 |

## 境界値チェックマトリクス

| 記号 | 意味 |
| --- | --- |
| ☑ | このテストファイルで明示的に確認している。 |
| ◐ | 近い条件は確認しているが、境界値そのものは直接確認していない。 |
| - | 未確認、またはこの境界は対象外。 |

### 文字列 / ID

| 対象値 / 条件 | empty | whitespace | valid existing | valid missing | malformed | other scope | deleted / inaccessible | max length + 1 / huge | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| userID | - | - | ☑ | ☑ | - | - | - | - | 全テスト | empty userID、長すぎる userID。 |
| email | - | - | ☑ | - | - | - | - | - | `TestSignInUser_NewUser_CreatesAccountAndGrantsCredit`, `TestSignInUser_ExistingUser_UpdatesLastLoginAndNoNewCredit` | empty email、invalid format、email 変更。 |
| displayName | ☑ | - | ☑ | - | - | - | - | - | 全テスト | whitespace only、max length。 |

### 状態 / 存在

| 対象値 / 条件 | absent | present | partial / orphan | duplicate | transition before | transition after | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| account row | ☑ | ☑ | ☑ | - | - | - | 全テスト | 同時ログイン race、account 作成失敗。 |

### 日時 / expiry

| 対象値 / 条件 | missing | past | just before | exactly at | just after | future | invalid format | 対応テスト | 不足 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| CreatedAt / LastLoginAt | - | ◐ | - | ☑ | - | - | - | 全テスト | clock skew、CreatedAt 不正 format。 |
