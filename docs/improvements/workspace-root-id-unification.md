# workspace_root item の id を workspace id に統一する

**Status:** 仕様ドラフト（実装前）
**採用方針:** 案A2（workspace_root id = workspace id）。既存データは DB リセットで破棄するため migration 不要。

## やりたいこと

ワークスペースの tree の根（`workspace_root` item）の id を、**その workspace の id と同じ値**にする。これにより「workspace paper」と「workspace_root item」の二重性を解消し、projection で毎回行っている付け替えを撤廃する。

## 現状の二重性（解くべき問題）

1 つのワークスペースの「根」が、2 つの別物として存在している。

| | workspace_root item | workspace paper |
|---|---|---|
| 出所 | DB（`tree_items`, `kind='workspace_root'`） | factory が合成（[workspacePaperFactory.tsx](../../apps/web/src/features/workspaces/paper/workspacePaperFactory.tsx)） |
| id | `rootItemId`（独立採番の ULID） | `workspaceId` |
| projection での扱い | **隠す**（[useWorkspaceProjection.ts:12](../../apps/web/src/features/workspaces/useWorkspaceProjection.ts#L12)） | 根 paper として出す |
| content | UI では未使用 | `WorkspacePaper`（アップロード欄・ジョブ一覧・document card） |

問題の核心は **id が 2 つある**こと（`workspaceId` と `rootItemId`）。projection は毎回この 2 つを橋渡しし、`document_root` の `parentId` を `rootItemId` → `workspaceId` に書き換えている（[useWorkspaceProjection.ts:17-22](../../apps/web/src/features/workspaces/useWorkspaceProjection.ts#L17-L22)）。この橋渡しは、リロード時の「ワークスペースが見つかりません」バグ（id の取り違え・タイミング競合）の温床でもあった。

## 採用案：A2（workspace_root id = workspace id）

workspace_root item を作るときの id 採番を `workspaceId` に固定する。

```
現状: wsID = newID(); rootItemID = newID();   ← 別々
採用: wsID = newID(); rootItemID = wsID;       ← 同一
```

`workspaceId` は既に永続化キー・URL・デバッグ API・`ExpansionMap` の主キーとして全層に根を張っているため、**workspace_root item をそこに合流させる**のが破壊が最小で、概念も素直（「workspace の根 item の id は workspace id」）。

### 検討した代替案（不採用）

- **案A1: 根 paper の id を `rootItemId` に寄せる** — `workspaceId` ベースの既存コード・永続化データが全て `rootItemId` ベースに変わり、互換が切れる。**不採用**。
- **案A3: projection は残し content の責務だけ整理** — id 二重が残り、提案の本質（workspace_root = tree の根）に応えていない。**不採用**。
- **案B: tree_item と Paper を型・永続ごと統合** — Paper は UI レイアウト都合の型（`hue`/`contentImportance`/`childMinShares`/`minWidth`）、tree_item は知識構造のドメインモデル（`governance_state`/`item_sources`/`kind`）。同一視すると UI 都合が DB に漏れ、vendored ライブラリの型変更がスキーマ変更を強制する。関心の分離が壊れる。**不採用**。

## これで消えるもの（純減する複雑性）

1. projection の `parentId` 付け替え（`it.parentId === workspaceRootItemId ? workspaceId : it.parentId`）→ DB の `parentId` がそのまま `workspaceId` を指すので不要。
2. `workspaceId` と `rootItemId` を別々に持ち回る経路の縮小（[useWorkspaceTree.ts](../../apps/web/src/features/workspaces/useWorkspaceTree.ts) の `runProjectWorkspacePapers(workspaceId, rootItemId)` 第2引数が `workspaceId` と一致）。
3. id 橋渡しに起因するバグの温床。

→ **workspace_root item がそのまま「tree の根」になる。**

## 一本化で新たに出る論点（実装前に潰す）

id を揃えると、**同じ id（= workspaceId）の paper が 2 経路で生成されうる**：

- projection が workspace_root item を paper 化しようとする
- factory の `buildWsPaper(workspaceId, ...)` も `workspaceId` の paper を生成する

→ **factory の `WorkspacePaper`（content にアップロード欄・ジョブ一覧・document card を持つ）を「根 paper」の正とし、projection 側は workspace_root item を引き続きスキップする。**
スキップ条件 `it.id === workspaceRootItemId`（[useWorkspaceProjection.ts:12](../../apps/web/src/features/workspaces/useWorkspaceProjection.ts#L12)）は `workspaceRootItemId === workspaceId` になっても**そのまま機能する**。

→ 実質的な簡素化は「**`parentId` 付け替えの撤廃**」と「**`rootItemId` を持ち回る必要の縮小**」に集約される。完全な「item = paper」化ではなく、「id を一致させて projection の付け替えを消す」一本化である点に注意。

## 前提

- 既存データは DB リセットで破棄する（本番データなし）。よって **migration 不要**、新規採番ルールだけ揃える。
- `document ↔ document_root` は `document_tree_links` の UNIQUE で 1:1（[0008_tree.up.sql:33-37](../../db/migrations/0008_tree.up.sql#L33-L37)）— 本件では変えない。
- tree は workspace ごとに 1 本（workspace_root は 1 個）という invariant は維持。

## 実装ステップ

### Phase 1 — backend 採番の統一
- [workspace.go:408](../../apps/api/internal/repository/postgres/workspace.go#L408): `rootItemID := newID()` → `rootItemID := wsID`。
- mock store [store.go:406](../../apps/api/internal/repository/mock/store.go#L406): `rootItemID := "nd_root_" + ...` → `rootItemID := w.WorkspaceID`。
- 既存テストで `rootItemId ≠ workspaceId` を前提にしているものがないか確認（あれば追従）。
- `AffectedWorkspaceRootItemID`（Firestore ジョブ通知、[firestore_job_status](../../internal/platform/job/status/firestore_job_status.generated.go#L117)）は常に workspace id と一致するようになる（コメントの「Almost always the same」が文字通り常に一致に）。

### Phase 2 — projection の付け替え撤廃
- [useWorkspaceProjection.ts:17-22](../../apps/web/src/features/workspaces/useWorkspaceProjection.ts#L17-L22): `projectedParentId` の三項書き換えを削除し、`it.parentId` をそのまま使う。
- スキップ条件（`it.id === workspaceRootItemId` で `return null`）は維持。
- `runProjectWorkspacePapers` / `projectWorkspacePapers` の `workspaceRootItemId` 引数は、`workspaceId` と同値になるが、スキップ判定にまだ使うため当面は残す（段階的に整理可能）。

### Phase 3 — フロント mock 生成の統一
- [workspaceTreeCache.ts:66](../../apps/web/src/features/workspaces/tree/workspaceTreeCache.ts#L66) 付近の mock tree 構築で、workspace_root item の id を `workspaceId` に揃える（デバッグ用 `createMockWorkspace` の挙動も実機と一致させる）。

### Phase 4 — DB リセットと動作確認
- DB をリセットして新規ワークスペースを作成。
- ドキュメントをアップロード → document_root が workspace paper 直下に並ぶこと、リロードで「見つかりません」が出ないことを確認。
- `__synthifyDebug.dumpWorkspace` で `rootItemId === workspaceId` を確認。

## 影響ファイル（見込み）

- `apps/api/internal/repository/postgres/workspace.go`（採番）
- `apps/api/internal/repository/mock/store.go`（mock 採番）
- `apps/web/src/features/workspaces/useWorkspaceProjection.ts`（付け替え撤廃）
- `apps/web/src/features/workspaces/tree/workspaceTreeCache.ts`（mock tree）
- 参考（変更小 or なし）: `apps/web/src/features/workspaces/useWorkspaceTree.ts`, `apps/web/src/features/tree/buildTree.ts`

## 対象外

- DB スキーマ・`kind` enum の変更（`workspace_root` kind は残す）。
- tree_item と Paper の型統合（案B）。
- cross-document tree（`cross_document` フラグ・`item_aliases`）。
