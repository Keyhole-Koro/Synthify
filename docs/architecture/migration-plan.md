# Migration Plan: From Current State to Target Design

[current-state.md](current-state.md) で書いた現状から、[target-design.md](target-design.md) で定義した「あるべき形」に到達するための PR の連なりを定義する。

## 前提

- **prod に本番ユーザーは居ない**。データはいつでも reset 可能。
- スキーマ変更は migrate-up 一発で完結させる。dual-write / バックフィル期間は設けない。
- 各 PR は **stage で動作確認 → 通れば main マージ → 自動デプロイで prod 反映** の単線。
- 失敗した PR は `git revert` か reset-prod-db でやり直し。

この前提があるため、本ドキュメントの構造は非常にシンプルになる。dual-write / read-fallback / 段階削除のような移行パターンは登場しない。

---

## 全体像 (PR の連なり)

| # | PR タイトル | スコープ | 依存 |
|---|---|---|---|
| 1 | `db: introduce tree_items.kind and document_tree_links` | スキーマ + backfill (空 DB なら no-op) | — |
| 2 | `proto: surface ItemKind on Item messages` | proto + 生成コード | 1 |
| 3 | `worker: write document_root_item with kind=document_root on persistence` | worker の item 作成ロジック | 1, 2 |
| 4 | `frontend: identify roots by kind instead of findRootItemId heuristic` | フロント | 2 |
| 5 | `joblifecycle: hoist Service into internal/platform/job/lifecycle` | コード移動 + interface 分割 | — |
| 6 | `joblifecycle: split QueuedNotifier and RuntimeController; enforce via DI` | api / worker の wiring 変更 | 5 |
| 7 | `pipeline: route stage updates through RuntimeController` | worker pipeline | 6 |
| 8 | `billing: move RecordUsage to platform package; worker writes directly` | パッケージ抽出 + worker 直接書き | — |
| 9 | `worker: stop calling api BillingService.RecordUsage` | NewConnectReporter 廃止 | 8 |
| 10 | `proto/firestore: extend job-status with createdDocumentRootItemId and reason` | スキーマ + notifier | 1〜3 |
| 11 | `worker: classify failure causes into JobFailureReason` | failJob リファクタ | 10 |
| 12 | `frontend: switch tree refresh to GetSubtree on completion` | フロント差分マージ | 10 |
| 13 | `worker: introduce JobOrchestrator interface and dispatch table` | worker 構造リファクタ | 6, 7 |
| 14 | `observability: emit JobCompleted custom event` | NR Custom Event 追加 | 6 |

依存グラフ (ざっくり):

```
1 ──► 2 ──► 3 ──► 4
      │            
      └──► 10 ──► 11 ──► 12
                              
5 ──► 6 ──► 7 ──► 13
        │             
        └──► 14       
                      
8 ──► 9
```

縦に並んでいるブロック (1-4, 5-7, 8-9, 10-12, 13, 14) は **互いに独立**。並行で進めても良いし、興味のある章から潰してもいい。

---

## PR 詳細

各 PR は「目的 / 変更 / 検証 / ロールバック」の 4 セクションで記述する。

### PR 1: tree_items.kind と document_tree_links

**目的**: 「workspace_root / document_root / node」をスキーマで明示。

**変更**:
- `db/migrations/00NN_tree_kind.up.sql`:
  ```sql
  ALTER TABLE tree_items
    ADD COLUMN kind text NOT NULL DEFAULT 'node'
      CHECK (kind IN ('workspace_root', 'document_root', 'node'));

  CREATE TABLE document_tree_links (
    document_id  text NOT NULL UNIQUE REFERENCES documents(document_id) ON DELETE CASCADE,
    root_item_id text NOT NULL UNIQUE REFERENCES tree_items(id) ON DELETE CASCADE,
    workspace_id text NOT NULL REFERENCES workspaces(workspace_id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now()
  );

  CREATE INDEX document_tree_links_workspace_idx
    ON document_tree_links(workspace_id);
  ```
- 対応する `.down.sql`
- `sqlc.yaml` / sqlc 生成の更新
- `domain.Item` に `Kind` フィールド追加

**検証**:
- `make logs-stage-api` でマイグレーション後に既存テストが通る
- `stage` で新ワークスペース作成 → 既存 `workspace_root` が `kind='node'` のままなのを確認 (PR 3 で修正される)

**ロールバック**: `migrate down` で `.down.sql` を当てて列削除。データが空ならノーリスク。

---

### PR 2: ItemKind を proto に露出

**目的**: フロントが `kind` を読めるようにする。

**変更**:
- `contracts/connectrpc/synthify/app/v1/tree_types.proto`:
  ```proto
  enum ItemKind {
    ITEM_KIND_UNSPECIFIED = 0;
    ITEM_KIND_NODE = 1;
    ITEM_KIND_WORKSPACE_ROOT = 2;
    ITEM_KIND_DOCUMENT_ROOT = 3;
  }

  message Item {
    // existing fields...
    ItemKind kind = N;
  }
  ```
- `buf generate` で Go / TS 出力を更新
- `apps/api/internal/handler` の `toProtoItem` に kind マッピング追加

**検証**: `GetTree` レスポンスに `kind` が乗ることを stage で curl 確認。

**ロールバック**: proto enum は後方互換的に追加するだけなので revert で問題なし。

---

### PR 3: worker が document_root_item を kind 付きで書く

**目的**: 新規 document 処理時に正しい `kind` を設定。

**変更**:
- `apps/worker/pkg/worker/tools/builtin/io/persistence.go` で document に対応する root item を作るときに `kind = 'document_root'` を設定
- workspace 作成時 ([apps/api/internal/repository/postgres/workspace.go](../../apps/api/internal/repository/postgres/workspace.go) で root item を生成する箇所) に `kind = 'workspace_root'` を設定
- `document_tree_links` への INSERT を同 transaction で
- 既存 item は migrate で全部 `kind='node'` だが、prod は空なので無視。stage に手作業のテストデータがあれば手動 update or reset

**検証**: アップロード → 完了後に Postgres を直接見て `tree_items.kind` と `document_tree_links` が正しいことを確認。

**ロールバック**: PR 単独 revert。次のジョブから旧挙動に戻る。

---

### PR 4: frontend が kind で root を識別

**目的**: `findRootItemId` 経験則の廃止。

**変更**:
- `apps/web/src/features/workspaces/useWorkspaceTree.tsx`:
  - `findRootItemId(items)` → `items.find(i => i.kind === 'ITEM_KIND_WORKSPACE_ROOT')`
  - `rootItem.childIds` → `items.filter(i => i.kind === 'ITEM_KIND_DOCUMENT_ROOT')` + 既存の parent_id チェック
- 古い `findRootItemId` ヘルパは削除

**検証**: ローカルでアップロード→tree 表示の挙動。stage で同様の手動テスト。

**ロールバック**: revert で旧 heuristic に戻る。

---

### PR 5: joblifecycle を internal/platform に昇格

**目的**: api と worker のコピペ解消。

**変更**:
- `internal/platform/job/lifecycle/service.go` を新規作成 (内容は現 worker 側を素直に移植)
- `apps/api/internal/job/lifecycle/` を削除
- `apps/worker/pkg/worker/job/lifecycle/` を削除
- 両 main.go の import を `internal/platform/job/lifecycle` に変更
- `Repository` interface を「両者が満たせる最小集合」に再定義 (`MarkProcessingJobRunning` 等)
- 既存テストの import path 更新

**検証**: `go test ./...` が全通過。stage で 1 ジョブ走らせて状態遷移が変わらないことを確認。

**ロールバック**: コード位置の移動だけなので revert で全戻し可能。

---

### PR 6: QueuedNotifier / RuntimeController の分離

**目的**: 「api は MarkRunning を呼べない」を型で強制。

**変更**:
- `internal/platform/job/lifecycle/service.go`:
  ```go
  type QueuedNotifier interface {
      NotifyQueued(ctx context.Context, payload jobstatus.Payload)
      RequestApproval(...) (...)
      ApproveApproval(...) (...)
      RejectApproval(...) (...)
  }
  type RuntimeController interface {
      MarkRunning(...)
      UpdateStage(...)
      Complete(...)
      TryFail(...)
  }
  ```
  `Service` 構造体は両 interface を実装
- `apps/api/internal/service/document.go` の `DocumentService.lifecycle` 型を `QueuedNotifier` に変更
- `apps/worker/pkg/worker/worker.go` の `Worker.lifecycle` 型を `RuntimeController` に変更
- これで api 側から `Complete` を呼ぶコードがあればコンパイルエラー (= 古い呼び出しを見つけられる)

**検証**: コンパイル + テスト。stage 動作確認。

**ロールバック**: revert で 1 interface 構造に戻る。

---

### PR 7: pipeline パッケージの削除 (元の計画から変更)

**当初の計画**: `pipeline.Runner` の直接 repo 呼びを `RuntimeController` 経由に置換。

**実際**: 着手時に `apps/worker/pkg/worker/pipeline` パッケージが **完全に dead code** であることが判明 (`NewRunner` を呼ぶ箇所が repo 全体にゼロ、テストも無し)。実際のステージ orchestration は `apps/worker/pkg/worker/agents` が担っていた。

**変更**: パッケージ丸ごと削除。これで「ステージ更新の経路を 1 つに集約」というゴール (RuntimeController しか経路がない状態) は満たされる。

**検証**: build と test の通過のみ。挙動変化なし。

**ロールバック**: revert で復活するが、復活させる意味のあるコードではない。

---

### PR 8: billing パッケージを platform に抽出

**目的**: api と worker 両方から `usage_events` を直接書けるようにする。

**変更**:
- `internal/platform/billing/usage/` を新規作成
- `apps/api/internal/service/billing.go` の `RecordUsage` ロジック (token 計算 / credit 控除 / daily 加算) を上記パッケージに切り出し
- `apps/api/internal/service/billing.go` は新パッケージを使う形に書き換え
- worker からの依存はまだ追加しない (PR 9 で)

**検証**: 既存のテストが全通過。api の RecordUsage 経路の挙動が変わっていないことを確認。

**ロールバック**: revert。

---

### PR 9: worker が Postgres に直接 RecordUsage

**目的**: worker → api Connect 経路の廃止。

**変更**:
- `apps/worker/pkg/worker/metering/reporter.go` の `NewConnectReporter` を廃止 (削除 or no-op 化)
- 代わりに `internal/platform/billing/usage` を使って Postgres に書く `PostgresReporter` を新設
- `apps/worker/cmd/server/main.go` の wiring 変更: BillingService client の構築を削除、Postgres reporter を注入
- `apps/api/internal/handler/billing.go` の `RecordUsage` ハンドラは残す (内部から呼び出される可能性 / 外部統合用)。ただし worker からの呼び出しは無くなる
- `SYNTHIFY_INTERNAL_SERVICE_TOKEN` env は当面残置 (将来別用途で使うかもしれない)

**検証**:
- stage で 1 ジョブ走らせて、`usage_events` テーブルに行が追加されることを Postgres で確認
- api の Cloud Run logs に `BillingService.RecordUsage` の呼び出しが来ないことを確認 (これが本 PR の本懐)

**ロールバック**: revert で Connect 経路復活。

---

### PR 10: 完了通知 payload の拡張

**目的**: フロントが追加 RPC なしで完了内容を理解できるように。

**変更**:
- `contracts/firestore/job-status.schema.json` に追加:
  - `createdDocumentRootItemId` (string, optional)
  - `affectedWorkspaceRootItemId` (string, optional)
  - `stageSummary` (array of strings, optional)
  - `reason` (enum, optional - PR 11 で値を確定)
- `scripts/generate-firestore-types.mjs` で生成型を更新
- `internal/platform/job/status/notifier.go` の `Completed` / `Failed` に新フィールドを書き込む引数を追加
- `lifecycle.Complete` / `lifecycle.TryFail` のシグネチャ拡張

**検証**: stage で完了 → Firestore ドキュメントを直接見て新フィールドが入っているか確認。

**ロールバック**: revert。下位互換 (フロントは未知フィールドを無視する) なので壊さない。

---

### PR 11: JobFailureReason の標準化

**目的**: `errorMessage` の文字列マッチを廃止。

**変更**:
- `apps/worker/pkg/worker/domain/job.go` に `JobFailure` 構造体と `JobFailureReason` enum 追加
- `apps/worker/pkg/worker/worker.go` の `failJob` を `JobFailure` を返す形に変更
- `failJob` 内で cause を分類:
  - "Publisher Model `...` was not found" を含む → `vertex_model_not_found`
  - `domain.ErrFileTooLarge` / `domain.ErrStorageQuotaExceeded` → `quota_exceeded`
  - agent パッケージから来た error → `agent_error`
  - context.Canceled → `cancelled`
  - その他 → `internal`
- `lifecycle.TryFail` で `JobFailure.Reason` を Firestore の `reason` フィールドに書く

**検証**: 既知の失敗ケース 2-3 種類を stage で再現し、reason の値を確認。

**ロールバック**: revert。

---

### PR 12: フロントを差分マージに

**目的**: tree 全取得をやめる。

**変更**:
- `apps/web/src/features/workspaces/useWorkspaceTree.tsx`:
  - `refreshWorkspaceTree(workspaceId, opts.revealNewDocumentRoots)` の経路を 2 つに分岐:
    1. 初回ロード: 既存の `getTree(workspaceId)` 経路
    2. 完了駆動: `getSubtree(workspaceId, createdDocumentRootItemId, 5)` + ローカル tree にマージ
- `WorkspacePaper.tsx` の `onProcessingComplete` で `jobStatus.createdDocumentRootItemId` を引いて新経路に渡す

**検証**: stage で 1 つアップロード → tree が全リフェッチではなく該当 subtree のみ取得していることを Network パネルで確認。

**ロールバック**: revert で `getTree` 全取得経路に戻る。

---

### PR 13: JobOrchestrator interface

**目的**: 新 job_type 追加時の触る場所を局所化。

**変更**:
- `apps/worker/pkg/worker/orchestrators/` を新規作成
- 現在の `Worker.Process` を `ProcessDocumentOrchestrator` に切り出し
- `Worker.Process` 自体は `map[appv1.JobType]JobOrchestrator` でディスパッチ
- `REPROCESS_DOCUMENT` も同じ枠組みに乗せる
- 既存テストの調整

**検証**: テスト + stage で 1 ジョブ走行。

**ロールバック**: revert。

---

### PR 14: JobCompleted Custom Event

**目的**: 成功率 / 所要時間を NR で見られるように。

**変更**:
- `internal/platform/job/lifecycle/service.go` の `Complete` で `nrApp.RecordCustomEvent("JobCompleted", ...)` を発行
- 属性: `job_id`, `job_type`, `workspace_id`, `document_id`, `duration_seconds`, `created_document_root_item_id`

**検証**: stage で 1 ジョブ走行 → NR の Insights で `SELECT * FROM JobCompleted SINCE 1 hour ago` で確認。

**ロールバック**: revert。

---

## 並行で進めるときの注意

1〜4 と 5〜7 と 8〜9 は **互いに独立**で同時並行 OK。ただし以下の交差点には注意:

- PR 3 と PR 9 がほぼ同時に main に入ると、worker が書く Postgres 列が増える。両 PR とも単独で動作するので問題ないが、マージコンフリクト発生時は `apps/worker/cmd/server/main.go` の wiring を慎重に
- PR 10 と PR 6 の同時マージは、`lifecycle.Complete` のシグネチャ変更が衝突する可能性。PR 6 を先にマージし、PR 10 を後にする方が安全

---

## やり残し / 将来の検討事項

- **dynamic_tools** の整理は対象外。target-design.md で扱っていないため
- **monitor 側のコード** はこの移行で触らない。monitor が読む `job_logs` テーブルは形を保つ
- **billing** の改修は今回「課金経路の物理的な書き手を変える」だけ。pricing 計算ロジック自体は変えない
- **`processing_jobs.completed_at` の独立カラム化** は今回やらない。完了時刻が必要なら `updated_at` を見る (現状維持)。これは Doc 1 で挙げた歪みの 1 つだが、フロント /API ともに表示用途のみで現状動作しているため緊急性なし

---

## 完了条件

すべての PR が main にマージされ stage と prod に反映され、以下が満たされたら本移行は完了:

1. `apps/{api,worker}/internal/job/lifecycle/` が **存在しない** (`internal/platform/job/lifecycle/` のみ)
2. `tree_items.kind` を見れば item の役割が分かる
3. `document_tree_links` を見れば document と root item の対応が分かる
4. フロントで `findRootItemId` ヘルパが **存在しない**
5. worker の Cloud Run logs に `BillingService.RecordUsage` の発信が無い
6. Firestore job-status ドキュメントが完了時に `createdDocumentRootItemId` と `reason` を持っている
7. `apps/worker/pkg/worker/orchestrators/` の下に少なくとも 1 つの実装がある
8. NR Insights で `JobCompleted` イベントが取れる

これらが揃った時点で `current-state.md` と本 `migration-plan.md` を **`docs/learn/` 配下に retrospective として移す** (改名 + 移動)。`target-design.md` は最終形の参照として `architecture/` に残す。
