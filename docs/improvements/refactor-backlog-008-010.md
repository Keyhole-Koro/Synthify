# Refactor Backlog 008–010 の整理

`refactor/README.md` に残っている次の 3 項目について、現状の実装との差分と、具体的にどう進めるべきかを整理する。

- `008: sqlc へのクエリ移行とリポジトリ層の整理`
- `009: ジョブ・ドキュメント状態遷移ロジックの集約`
- `010: テスト用モック・データ生成ファクトリの共有化`

## 結論

- `008` は実質完了扱いでよい。残件は `joblog.go` 周辺の生 SQL のみ。
- `009` は未完。状態遷移が service / worker / repository に分散している。
- `010` は未完。テスト fixture helper が複数箇所に重複している。

## 008: sqlc へのクエリ移行とリポジトリ層の整理

### 現状

- repository の主要系統はすでに `sqlc` ベースになっている。
- `sqlc.yaml` は `db/queries` 全体を対象に `packages/shared/repository/postgres/sqlcgen` を生成している。
- `Store` も `sqlcgen.Queries` を中心に組まれている。

### 残っている問題

- `packages/shared/repository/postgres/joblog.go` に `ExecContext` / `QueryContext` を使った生 SQL が残っている。
- 特に related jobs のクエリは `relatedJobsSQL()` で動的に文字列組み立てしている。

### やること

1. `job_logs` 系の `INSERT` と単純な `SELECT` を `db/queries` 配下へ移す。
2. related jobs 取得は 1 本の動的 SQL に寄せず、用途別に分ける。
3. `packages/shared/repository/postgres` 直下の `db.ExecContext` / `db.QueryContext` を原則ゼロにする。

### 具体案

- `db/queries/joblogs.sql` を追加する。
- 例えば以下のように分ける。
  - `InsertJobLog`
  - `ListJobLogsByJob`
  - `ListRelatedJobsByJob`
  - `ListRelatedJobsByDocument`
  - `ListRelatedJobsByWorkspace`
- `unifiedJobLogsSourceSQL()` のような複雑な union は、無理に先に移さず後回しでもよい。

### 完了条件

- `joblog.go` の単純 CRUD が `sqlc` 経由になる。
- repository 層で生 SQL を使うのは、`sqlc` に載せにくい特殊クエリだけになる。

## 009: ジョブ・ドキュメント状態遷移ロジックの集約

### 現状

- job の queued / running / failed / succeeded 更新が複数レイヤに散っている。
- plan の pending_approval / approved / rejected 更新も repository メソッド内に個別実装されている。
- document の lifecycle state は proto に存在するが、実際には一元管理されていない。
- mapper では document status が常に `UPLOADED` 固定になっている。

### 問題点

- 同じ失敗処理や notifier 呼び出しが `DocumentService` と worker で重複している。
- 状態遷移の正しさを 1 箇所で検証できない。
- 「job 状態」「plan 状態」「document 状態」の責務分界が曖昧。

### やること

1. 状態遷移の入口を 1 箇所に寄せる。
2. service / worker は「遷移を決める」のではなく「遷移 API を呼ぶ」だけにする。
3. repository は永続化専用に寄せる。
4. document 状態の決定ルールを明文化する。

### 具体案

新しい集約先は `packages/shared/domain` 直下でも `packages/shared/service` 直下でもよいが、少なくとも次の責務を持たせる。

- `QueueJob`
- `MarkJobRunning`
- `FailJob`
- `CompleteJob`
- `RequestApproval`
- `ApprovePlan`
- `RejectPlan`

各メソッドで以下をまとめて扱う。

- job status の更新
- plan status の更新
- 必要なら document status の更新
- notifier 呼び出し
- joblog 記録

### 段階的な進め方

#### Phase 1

- queued / running / failed / succeeded の job 遷移だけ先に集約する。
- `apps/api/internal/service/document.go` と `apps/worker/pkg/worker/worker.go` の重複失敗処理を除去する。

#### Phase 2

- approval request / approve / reject と plan status 遷移を集約する。
- `packages/shared/repository/postgres/document.go` の承認関連トランザクションを薄くする。

#### Phase 3

- document lifecycle state の導出ルールを決める。
- 例:
  - job が queued/running のとき `PROCESSING`
  - approval 待ちのとき `PENDING_NORMALIZATION`
  - job 成功で `COMPLETED`
  - job 失敗で `FAILED`
- `mappers.ToProtoDocument` の固定値をやめる。

### 完了条件

- service / worker から repository の状態更新メソッドを直接多重に呼ばなくなる。
- job / plan / document の状態遷移ルールが 1 箇所で読める。
- document status が proto 定義どおり意味を持つ。

## 010: テスト用モック・データ生成ファクトリの共有化

### 現状

- workspace 作成 helper が複数の test file に重複している。
- workspace + tree + document + job を作る helper も個別に生えている。
- 命名と戻り値が揃っていないため、テスト追加時に毎回同じ初期化を書くことになる。

### 問題点

- テストの前提条件がファイルごとに微妙にズレやすい。
- helper の改善を一括反映できない。
- `mock.Store` を使う API/service/repository テストで同じ組み立てを繰り返している。

### やること

1. 共通 fixture helper の置き場所を決める。
2. 文字列だけ返す helper をやめ、構造体で返す。
3. 既存のローカル helper を順次置き換える。

### 具体案

配置候補:

- `packages/shared/testutil`
- もしくは `packages/shared/repository/mock/testutil`

最低限ほしい helper:

- `CreateUserWorkspace`
- `CreateWorkspaceWithTree`
- `CreateWorkspaceWithDocument`
- `CreateWorkspaceWithProcessingJob`

戻り値は次のような fixture struct にする。

```go
type WorkspaceFixture struct {
	UserID      string
	WorkspaceID string
	TreeID      string
	DocumentID  string
	JobID       string
}
```

必要なら `AccountID`, `Document`, `Job`, `Tree` 本体も保持してよい。

### 置換対象

- `apps/api/internal/handler/authz_test.go`
  - `setupWorkspaceInStore`
  - `setupItemFixturesInStore`
- `apps/api/internal/service/workspace_test.go`
  - `createWorkspaceForUser`
- `packages/shared/repository/mock/store_test.go`
  - `setupWorkspace`
  - `setupTree`

### 完了条件

- workspace/tree/document/job を作る helper が 1 系統に揃う。
- 新しいテストで ad-hoc な seed helper を増やさなくてよくなる。

## 推奨実施順

1. `008` は improvement としては低コスト完了候補。README 上もクローズ寄りでよい。
2. `010` を先にやる。差分が小さく、後続のテスト追加が楽になる。
3. `009` を最後にやる。まず job 遷移だけを集約し、document 状態までを一気に抱え込まない。
