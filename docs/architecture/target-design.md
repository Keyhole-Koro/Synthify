# Target Design: Vocabulary, Responsibilities, and Contracts

[current-state.md](current-state.md) で記述した歪みを解消するための「あるべき形」を定義する。実装には踏み込まず、**「この設計に従って書けば、読み手はコードを 1 回見ただけで挙動を理解できる」** を目標にする。

ゴール: 読みやすさ / チーム作業でのバグ耐性。スケールビリティと最適化は副次的な検討事項。

実装の順序は [migration-plan.md](migration-plan.md) (執筆予定) で扱う。

---

## 0. 設計原則

1. **唯一の真実 (Single Source of Truth) を明示する** — どのレイヤがどのデータを所有するかを章 4 で固定する。これ以外のレイヤはミラー / キャッシュ / 派生と位置付ける。
2. **責務の中心を 1 つにする** — 同じ概念を扱うコードが複数のパッケージにコピペされている状態を許さない。共通化できないなら境界を引き直す。
3. **暗黙の慣習を語彙にする** — 「フロントだけが知っている呼び名」「`parent_id IS NULL` で root と判定」のような構造を、すべて型 / カラム / enum / 命名で見える化する。
4. **min-instances=0 を許容する** — api Cloud Run はユーザー操作が無いとき寝てよい。worker は api に同期で依存しない。

---

## 1. 語彙

### 1.1 確定する用語

| 用語 | 意味 |
|---|---|
| **workspace** | アカウントが持つプロジェクト単位。1 つの workspace は 1 つの workspace_tree を持つ |
| **workspace_tree** | workspace 内の知識グラフの全体。「tree」単独の語は使わない |
| **workspace_root_item** | workspace_tree のルート。1 workspace に必ず 1 つ存在し、`parent_id IS NULL` |
| **document_root_item** | document に対応する 1 階層下の item。`workspace_root_item` の直下の子。1 document = 1 document_root_item |
| **item** | workspace_tree の任意のノード。種別は `kind` カラムで識別 |
| **subtree** | 任意の item を root とする部分木 |

### 1.2 廃止する用語

- 単独の「**tree**」は使わない。API 上の `Tree { tree_id, ... }` も廃止。tree_id カラム / フィールドは workspace_id に置き換える
- 「**root**」とだけ書かれた変数 / フィールドは禁止。常に `workspace_root_item` か `document_root_item` を指定する

### 1.3 `tree_items.kind` 列の導入

`tree_items` に新カラム `kind` (enum) を追加:

```sql
ALTER TABLE tree_items
  ADD COLUMN kind text NOT NULL DEFAULT 'node'
    CHECK (kind IN ('workspace_root', 'document_root', 'node'));
```

制約 (アプリケーション層で保証):
- 1 workspace につき `kind = 'workspace_root'` の item はちょうど 1 つ
- `kind = 'document_root'` の item は `parent_id = workspace_root_item.id` を満たす
- `kind = 'document_root'` の item は 1 document に対して 1 つ (詳細は 1.4)

### 1.4 `document_root_item` と `documents` の対応

新テーブル `document_tree_links` で 1:1 を保証:

```sql
CREATE TABLE document_tree_links (
  document_id      text NOT NULL UNIQUE REFERENCES documents(document_id) ON DELETE CASCADE,
  root_item_id     text NOT NULL UNIQUE REFERENCES tree_items(id) ON DELETE CASCADE,
  workspace_id     text NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
  created_at       timestamptz NOT NULL DEFAULT now()
);
```

`UNIQUE` を双方向に張り、1 document = 1 root_item を schema で保証する。`item_sources` (証拠リンク) は残し、それとは別レイヤ。

### 1.5 frontend / proto / Go の整合

- proto: `Item { id, parent_id, label, ..., kind }` に `kind` (enum `ItemKind { ITEM_KIND_NODE, ITEM_KIND_WORKSPACE_ROOT, ITEM_KIND_DOCUMENT_ROOT }`) を追加
- フロントの `findRootItemId(items)` は `items.find(i => i.kind === 'workspace_root')` に置換
- フロントの `documentRootIds` 抽出は `items.filter(i => i.kind === 'document_root')` に置換

---

## 2. ジョブの責務分担

### 2.1 責務マトリクス

| 操作 | 主 (書き手) | 補 (読み手) |
|---|---|---|
| ジョブの作成 (`queued`) | **api** | worker |
| `running` への遷移 | **worker** | api (Postgres から read のみ) |
| ステージ更新 (`current_stage`) | **worker** | api |
| `succeeded` への遷移 | **worker** | api |
| `failed` への遷移 (実行中の失敗) | **worker** | api |
| `failed` への遷移 (dispatch RPC 自体の失敗) | **api** | worker |
| 承認リクエスト (approval) | **api** | worker |
| 承認の承認 / 却下 | **api** | worker |
| 課金記録 (`usage_events` 等) | **worker** (直接 Postgres) | api (照会のみ) |
| Firestore 通知 | **状態遷移を起こしたサービス** | フロント |

`failed` 遷移が両者にあるのは意図的だ。worker が呼ばれる前に api → worker の RPC が落ちたケースは、その時点で worker は事象を知らない。api が `TryFail` を呼んで状態を確定させる責任を持つ。

**原則: api min-instances=0 を許容するため、worker は処理を完走するために api に同期依存しない。**

### 2.2 `joblifecycle.Service` の統合

現状 api と worker にコピペで存在する `joblifecycle.Service` を `internal/platform/job/lifecycle/` に 1 つだけ置く。

```go
// internal/platform/job/lifecycle/service.go
package joblifecycle

type Service struct {
    repo     Repository      // 共通 interface
    notifier jobstatus.Notifier
    logger   *slog.Logger
    nrApp    *newrelic.Application
}
```

`Repository` interface は両サービスから満たせる最小限のメソッドだけ定義する:

```go
type Repository interface {
    MarkProcessingJobRunning(ctx context.Context, jobID string) error
    UpdateProcessingJobStage(ctx context.Context, jobID, stage string) error
    FailProcessingJob(ctx context.Context, jobID, errorMessage string) error
    CompleteProcessingJob(ctx context.Context, jobID string, completedRoot CompletedRoot) error
    RequestJobApproval(...) (...)
    ApproveJobApproval(...) (...)
    RejectJobApproval(...) (...)
}
```

api と worker の `postgres.Store` がそれぞれこの interface を実装する。

### 2.3 責務境界の明文化

呼び出し主体ごとのルール:

- **api 側 (DocumentService)**:
  - `lifecycle.NotifyQueued` のみ呼ぶ
  - `lifecycle.MarkRunning`, `Complete`, `TryFail` は **絶対に呼ばない**
  - 承認系メソッドは api 側だけ
- **worker 側 (Worker.Process)**:
  - `lifecycle.MarkRunning`, `Complete`, `TryFail` を呼ぶ
  - `pipeline.Runner` 内からの `UpdateProcessingJobStage` は `lifecycle.UpdateStage` 経由に統一 (現状の直接 repo 呼びは禁止)

「`MarkRunning` を api から呼ばない / `NotifyQueued` を worker から呼ばない」を **コンパイル時に保証する**ため、`lifecycle.Service` を 2 つの interface に分割する:

```go
type QueuedNotifier interface {
    NotifyQueued(ctx context.Context, payload jobstatus.Payload)
    TryFail(ctx context.Context, payload jobstatus.Payload, cause error)
}

type RuntimeController interface {
    MarkRunning(ctx context.Context, payload jobstatus.Payload) error
    UpdateStage(ctx context.Context, payload jobstatus.Payload, stage string) error
    Complete(ctx context.Context, payload jobstatus.Payload) error
    TryFail(ctx context.Context, payload jobstatus.Payload, cause error)
}
```

`Service` 構造体は両方を実装するが、api の DocumentService には `QueuedNotifier` のみ注入し、worker の `Worker` には `RuntimeController` のみ注入する。これで「api が誤って `Complete` を呼ぶ」コードは型エラーになる。

`TryFail` を両 interface に置く理由は責務マトリクスのとおり: dispatch RPC 自体が失敗したケースは api だけが状態を確定できる。

approval メソッド (`RequestApproval` / `ApproveApproval` / `RejectApproval`) は lifecycle には含めない。これらは Repository への単純なパススルーで、`JobHandler` から直接 repo を呼べばよく、lifecycle を経由しても何も得ない。worker 側では使わないので dead code を生むだけだった。

### 2.4 課金経路の見直し

worker → api の Connect 経由 `BillingService.RecordUsage` は **廃止する**。

代わりに:
- worker 内に新パッケージ `metering/billing` を作り、`Postgres に直接書く UsageWriter` を実装
- 既存の `apps/api/internal/service/billing.go` の `RecordUsage` ロジックを `internal/platform/billing/usage/` に昇格 (api と worker から共有)
- api 側の `BillingService.RecordUsage` RPC は当面残すが「フロント / 外部からの記録経路」用途に限定。worker は使わない

これで worker は self-contained になり、api min-instances=0 でもジョブ完走と課金記録が両立する。

### 2.5 Pipeline Runner との関係

現状 `pkg/worker/pipeline/runner.go` がステージ更新と `CompleteProcessingJob` を独自に呼んでいる。これは `lifecycle.RuntimeController` 経由に統一する。pipeline は **ステージの順序制御のみ責任を持ち、Postgres / Firestore 書き込みは lifecycle に委譲する**。

---

## 3. ジョブ完了通知の payload

### 3.1 何を運ぶか

完了 (および進行中の重要イベント) の通知は、フロントが「何が起きたか」「次に何を読めばいいか」を **追加 RPC を呼ばずに**理解できる情報を含める。

ジョブ完了時の Firestore 通知ドキュメント (拡張):

| フィールド | 既存/新 | 型 | 意味 |
|---|---|---|---|
| `status` | 既 | enum | succeeded / failed |
| `currentStage` | 既 | string | 完了時は "" |
| `progress` | 既 | int | 100 |
| `message` | 既 | string | "Completed" / "Failed" |
| `errorMessage` | 既 | string | 失敗時のみ |
| `completedAt` | 既 | RFC3339 | |
| `expiresAt` | 既 | Timestamp | now+7d |
| **`createdDocumentRootItemId`** | **新** | string | 今回作成された document_root_item の id (PROCESS_DOCUMENT 完了時) |
| **`affectedWorkspaceRootItemId`** | **新** | string | workspace_root_item の id (常に同じだが冪等性のため明示) |
| **`stageSummary`** | **新** | string[] | 通過したステージのリスト (デバッグ用) |
| **`reason`** | **新 (failed のみ)** | enum | failure の分類 (`vertex_model_not_found` / `quota_exceeded` / `agent_error` / `internal` / `cancelled`) |

`reason` enum を導入することで、フロントが `errorMessage` の文字列マッチで分類するコードを書かなくて済む。

### 3.2 reason の標準化

worker の `failJob` から `lifecycle.TryFail` に渡す `error` を `domain.JobFailure` に統一:

```go
type JobFailure struct {
    Reason  JobFailureReason
    Message string
    Cause   error
}

type JobFailureReason string

const (
    JobFailureReasonVertexModelNotFound JobFailureReason = "vertex_model_not_found"
    JobFailureReasonQuotaExceeded       JobFailureReason = "quota_exceeded"
    JobFailureReasonAgentError          JobFailureReason = "agent_error"
    JobFailureReasonCancelled           JobFailureReason = "cancelled"
    JobFailureReasonInternal            JobFailureReason = "internal" // 既知パターンに当てはまらない
)
```

`worker.failJob` は cause を解析して `Reason` を判定する責務を持つ。フロントは `reason` enum を switch するだけでよくなる。

### 3.3 Connect RPC の戻り値

`ExecuteApprovedPlan` のレスポンスを `{status: "ok"}` から拡張:

```proto
message ExecuteApprovedPlanResponse {
  string status = 1;
  string created_document_root_item_id = 2;
  repeated string created_item_ids = 3;
}
```

ただし **api がこれを使うのは「ジョブを同期実行している場合」のみ**。実運用では Firestore 通知が一次経路、Connect レスポンスは副次扱い。

---

## 4. 状態同期の契約

### 4.1 SoT の定義

| 種別 | SoT | mirror / 派生 |
|---|---|---|
| ジョブ状態 (`status`, `current_stage`) | **Postgres `document_processing_jobs`** | Firestore `workspaces/{ws}/jobs/{jobId}`、Connect `Job` proto |
| アイテム / ツリー構造 | **Postgres `tree_items` 系** | Connect `Item` proto |
| 課金イベント / 使用量 | **Postgres `usage_events` / `account_usage_daily`** | api の集計 RPC レスポンス |
| アカウント / ワークスペース | **Postgres** | Firebase Auth は authentication のみ、認可は Postgres |
| ドキュメントオブジェクト本体 | **GCS** | Postgres `documents` はメタデータのみ |

### 4.2 Postgres → Firestore のミラーリング契約

ルール:
1. **書き込みは必ず Postgres → Firestore の順**。Firestore に書いてから Postgres を書く経路を作らない
2. **Firestore 失敗は Postgres を巻き戻さない** (現状維持)。ログを残してジョブは続行
3. **Firestore は最終的整合性 (eventual consistency)** であることを doc に明記。フロントは Firestore の値を「直近 1〜数秒の状態」と理解する
4. **冪等性**: Firestore `MergeAll` で書く。同じ状態を 2 回書いても問題ない
5. **再起動時のリカバリ**: api / worker 起動時にも特別な reconcile は行わない。次のジョブイベントで自然に同期される

### 4.3 ドリフトの許容範囲

| シナリオ | 許容するか | 補足 |
|---|---|---|
| Firestore が Postgres より古い | ○ | UI に短時間表示遅延として出る |
| Firestore が Postgres より進んでいる | × | コード上発生しえない (Postgres 先書き) |
| Firestore に書かれていないジョブが Postgres に存在 | △ | フロントがそのジョブを表示できないだけ。バッチで補修可能 |

### 4.4 「Postgres が真」の運用上の含意

- ユーザー Support 対応時: Postgres を見る (Firestore は古い可能性)
- 障害時の reconcile: Postgres から Firestore に再書き込みする batch job を用意できるが、当面不要
- 削除: Firestore TTL (7 日) で表示用 mirror は自然に消える。Postgres のジョブ行は永続 (監査用)

---

## 5. 拡張点: 新 job_type を追加する手順

### 5.1 現状の問題

新しい `JobType_*` を追加するときに、変更が必要な箇所が散らばっている (proto / Go / Postgres / Firestore / worker handler / api handler / フロント)。

### 5.2 拡張点を 1 箇所に集める

新 job_type を追加する場合、触る場所を以下に限定する:

1. **proto**: `JobType` enum に 1 行追加
2. **worker**: 新 `JobOrchestrator` 実装を `apps/worker/pkg/worker/orchestrators/` に追加し、`worker.go` のディスパッチテーブルに 1 行追加
3. **api**: なし (新 job_type は worker が完全に責任を持つ)
4. **フロント**: 任意。enum を switch している箇所があれば case を追加

`JobOrchestrator` interface:

```go
type JobOrchestrator interface {
    JobType() appv1.JobType
    Process(ctx context.Context, req ExecutePlanRequest) (JobResult, error)
}
```

`Worker.Process` 内のディスパッチは `map[appv1.JobType]JobOrchestrator` で動的解決。これで新 job_type 追加が「1 ファイル新規 + 1 行登録」で済む。

### 5.3 既存実装の整理

現在の `Worker.Process` の中身を `ProcessDocumentOrchestrator` として切り出す。`ReprocessDocumentOrchestrator` は既存ロジックをラップする形で導入し、最終的に統合可否を判断する。

---

## 6. API 粒度

### 6.1 現状の問題

フロントがジョブ完了時に tree 全体を再取得している。新しい document が追加されただけでも `GetTree(workspaceId)` で全 item を読み直す。

### 6.2 差分取得 API の整理

完了通知の `createdDocumentRootItemId` (章 3.1) と既存の `TreeService.GetSubtree(workspaceId, itemId, maxDepth)` を組み合わせる:

```typescript
// frontend pseudo-code
firestoreListener(jobStatus => {
  if (jobStatus.status === 'succeeded' && jobStatus.createdDocumentRootItemId) {
    const subtree = await getSubtree(workspaceId, jobStatus.createdDocumentRootItemId, 5);
    mergeIntoLocalTree(subtree);
  }
});
```

新 RPC を追加するのではなく、既存の `GetSubtree` を活用する。

### 6.3 `TreeService.GetTree` の使い分け

| 用途 | 推奨 RPC |
|---|---|
| 初回ロード (workspace 全体) | `GetTree(workspaceId)` |
| ジョブ完了後の差分更新 | `GetSubtree(workspaceId, createdDocumentRootItemId, depth)` |
| ユーザーが item を展開 | `GetSubtree(workspaceId, expandedItemId, 1)` |
| パス検索 | `FindPaths(...)` |

`GetTree` は「初回のみ呼ぶ重い RPC」と doc で位置付けし、フロントは差分マージを基本にする。

---

## 7. 観測 (ログ / メトリクス)

### 7.1 既存の仕組み

- `slog` を JSON で stdout → Cloud Logging
- New Relic APM (Connect interceptor で自動収集)
- New Relic Custom Event (`JobFailed`, `UploadRejected`, `UploadSizeMismatch`, `OrphanObjectDeleteFailed`)
- Postgres `job_logs` テーブル (Monitor UI 用)

### 7.2 追加する観測点

- `JobCompleted` Custom Event を NR に送る (`JobFailed` と対称) — reason 別 / job_type 別の成功率と所要時間
- worker の `connect.handler_failed` ログ (既存) と合わせて、`job.dispatch_failed` のような cross-service 相関は trace_id で追える
- 「Postgres / Firestore のドリフト検出」は当面実装しない (障害頻度が問題になれば後付け)

### 7.3 削減する観測点

- `job_logs` (Postgres テーブル) は **Monitor UI が読む** ためにだけ残す。普段の開発者は Cloud Logging を一次経路にする
- `job_mutation_logs` は監査用途として残す

---

## 8. 設計原則とのマッピング

このドキュメントの各章が冒頭の 4 つの設計原則をどう実現しているかを確認:

| 原則 | 章 |
|---|---|
| 唯一の真実 (SoT) を明示する | 章 4 (Postgres が真、Firestore は mirror) |
| 責務の中心を 1 つにする | 章 2 (lifecycle 統合、reason 標準化)、章 5 (JobOrchestrator) |
| 暗黙の慣習を語彙にする | 章 1 (kind 列、document_root_item)、章 3 (reason enum) |
| min-instances=0 を許容する | 章 2.4 (課金経路の見直し) |

---

## 9. やらないこと

このドキュメントの **対象外**:

- スケーリング (worker の並列実行など)
- 認可モデルの変更 (`account_users.role` まわり)
- multi-tenancy / workspace 共有
- audit / compliance 要件
- フロントの状態管理ライブラリ選定

これらは別途検討する。本ドキュメントは **「コードが読みやすく、新しい開発者が変更を加えてもバグが入りにくい状態」** を作ることだけが目的。

---

## 10. 次のステップ

[migration-plan.md](migration-plan.md) で、現在の状態からこの target design に到達する PR の連なりと、各段階で `stage` が壊れない移行手順を定義する。

特に難易度が高い遷移:
- `tree_items.kind` 列の追加と `document_tree_links` の同時導入 (既存 workspace のバックフィル)
- 課金経路の worker 直接書きへの切り替え (api Connect 経路を残しつつデュアル書きで段階移行)
- `joblifecycle.Service` の `internal/platform/job/lifecycle/` への統合 (api / worker 同時に切り替える必要)

これらは migration-plan.md で個別に手順化する。
