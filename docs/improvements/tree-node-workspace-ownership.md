# node を workspace 直属の統合ツリーにする（document 従属の解消）

**Status:** 設計ドラフト（実装前・大きな構造転換）
**採用方針:** tree の node は document に従属せず、**workspace が持つ 1 本の統合ツリー**とする。document はツリーの頂点（`document_root`）ではなく、`item_sources` 経由の**出典**にすぎない。`document_root` / `workspace_root` / `kind` を廃止する。

## 問題：node が document に従属している

現状、知識ノード（`node`）が `document_root` の傘下に置かれ、**文書ごとにツリーが分断**されている。

```
現状のツリー:
workspace_root
├─ document_root (論文A)   ← document ごとに頂点が分かれる
│   ├─ node (序論)          ← この node は「論文A の node」に固定される
│   └─ node (提案手法)
└─ document_root (論文B)
    └─ node (結論)          ← 論文A と論文B の知識が統合されない
```

論文A の「序論」と論文B の「序論」は、概念的に同じでも**別々の node**になる。文書の壁が知識統合を妨げている。これは「workspace の知識を 1 本のツリーに統合する」という Synthify の目的に反する。

## あるべき姿：node は workspace の統合ツリー、document は出典

```
あるべきツリー:
workspace「研究ノート」          ← ツリーの根 = workspace 自身
├─ node「序論」                  ← workspace の統合ツリーの一部（document 非依存）
├─ node「提案手法」
│   └─ node「アルゴリズム」
└─ node「結論」

  document(論文A/B) は item_sources 経由で各 node の「出典」として横から紐づく。
  例: node「序論」← 論文A も論文B も出典（cross_document = true）
```

**node の親は常に `node`（または workspace）であり、`document_root` を経由しない。document は node の「親」ではなく「出典」。**

## この設計を支える部品は既にある（思想は最初から仕込まれていた）

- **`item_sources`**（[0008_tree.up.sql](../../db/migrations/0008_tree.up.sql)）— `item_id ⟷ document_id/file_id/chunk_id` の出典関係。`PRIMARY KEY (item_id, document_id, chunk_id)` で **1 つの node が複数 document を出典に持てる**。これが「document は出典」の中核。
- **`cross_document` フラグ**（proto に存在、[tree_types_pb.d.ts:111](../../apps/web/src/gen/proto/synthify/app/v1/tree_types_pb.d.ts#L111)）— node が文書をまたぐ統合ノードか。現状**未配線**だが概念は用意済み。
- **`item_aliases` + ApproveAlias/RejectAlias**（[item.go](../../apps/api/internal/service/item.go)）— 文書ごとにできた node を「同じ概念」として統合（dedup/merge）する仕組み。部品のみ存在。

→ これらが活きるのは node が document 非従属になってから。現状の `document_root` 構造がこれらの足を引っ張っている。

## ER 差分

| 項目 | 現状 | あるべき姿 |
|---|---|---|
| **node の親** | `document_root`（document 従属） | `node` または `workspace`（document 非依存） |
| **ツリーの本数** | document ごとに subtree が分かれる | **workspace に 1 本の統合ツリー** |
| **document の役割** | ツリーの一部（`document_root`） | **出典のみ**（`item_sources` 経由） |
| **`document_tree_links`** | document ⟷ document_root の 1:1 | **廃止** |
| **`document_root` kind** | 存在 | **廃止**（document はツリーノードではない） |
| **`workspace_root` kind** | 存在（workspaces の影） | **廃止**（workspace 自身が根） |
| **`kind` 列** | 3 値（workspace_root/document_root/node） | **不要**（node しか残らない） |
| **知識統合** | 文書の壁で分断 | `cross_document` + `item_aliases` で文書横断統合 |

### あるべき姿の ER（中核）

```
workspaces ──< tree_items (parent_id 自己参照のみ、kind なし)
                   │
                   └──< item_sources >── documents / document_files / document_chunks
                        (node の出典。1 node : N source)
```

- `tree_items.parent_id`：`workspace` 直下の node は workspace を、それ以外は親 node を指す（id 統一で workspace_id と item id を同一 ID 体系にすれば、parent_id は単一カラムで両方を表せる。FK の張り方は実装時に詰める）。
- `document_tree_links` 廃止、`tree_items.kind` 廃止。

## 影響範囲（大きい）

### backend / worker — tree 生成の作り直し
- [persistence.go:60-121](../../apps/worker/pkg/worker/tools/builtin/io/persistence.go#L60-L121): 現在は `GetWorkspaceRootItemID` → `CreateDocumentRootItemWithCapability` で document_root を頂点に作り、その下に node をぶら下げ、`UpsertItemSource` で出典を記録している。あるべき姿では **document_root を作らず、node を直接 workspace ツリーに接ぐ**（既存 node への統合 or 新規 node 作成）。`UpsertItemSource` は維持（むしろ主役に）。
- `CreateDocumentRootItem*` / `CreateDocumentTreeLink` / `GetWorkspaceRootItemID` 系の API を廃止・置換。
- node の「どこに接ぐか」= 統合ロジック（既存 node との同一性判定）が新たに必要。`item_aliases` / `cross_document` の配線がここで効く。

### DB スキーマ
- `tree_items.kind` 列と CHECK 制約を削除。
- `document_tree_links` テーブルを削除。
- 開発中につき DB リセットで対応（migration 不要、[workspace-root-id-unification](workspace-root-id-unification.md) と同方針）。

### frontend — projection / buildTree の簡素化
- [buildTree.ts:38,46](../../apps/web/src/features/tree/buildTree.ts#L38): `findRootItemId`（kind=WORKSPACE_ROOT）/ `findDocumentRootItemIds`（kind=DOCUMENT_ROOT）が**不要に**。根は workspace、頂点 node は workspace 直下の node。
- [useWorkspaceProjection.ts](../../apps/web/src/features/workspaces/useWorkspaceProjection.ts): workspace_root の「隠す・付け替え」が消える（[workspace-root-id-unification](workspace-root-id-unification.md) で先行して撤廃する分とも整合）。
- ワークスペース paper の content に並ぶ「document カード」は、ツリーノードではなく **document 一覧（出典の供給源）** を出すものに意味が変わる。document を開くと「その document を出典に持つ node 群」を見せる、等の再設計余地。

### proto / 生成コード
- `ItemKind` enum の廃止 or 縮小、`document_root` を返す API の見直し。buf 再生成（正バージョン）。

## 関連ドキュメント

- [workspace-root-id-unification.md](workspace-root-id-unification.md) — workspace_root id = workspace id。本設計の **前段**（workspace 自身が根になる準備）。本設計はさらに踏み込み、document_root も廃して node を workspace 直属にする。
- [tree-item-kind-renaming.md](tree-item-kind-renaming.md) — kind 改名案。本設計が実現すると **kind 列自体が不要**になるため、改名ではなく廃止に発展する。本設計を採るなら kind-renaming は不要になる。

## 未決定事項（実装前に詰める）

1. **node の統合判定** — アップロードした document を解析した結果を、既存ツリーのどの node に接ぐ／統合するか。完全新規か、既存 node への merge か。LLM 判定 + `item_aliases` 承認フローの設計。
2. **parent_id の表現** — workspace 直下 node の parent をどう持つか（workspace_id を指す／NULL + workspace_id 所属、など）。FK と自己参照の整合。
3. **document UI の役割** — document_root が消えた後、UI で document をどう見せるか（出典一覧・document ごとの寄与 node ハイライト等）。
4. **既存の document 単位ジョブ・課金との整合** — アップロード = document = ジョブ単位は維持しつつ、その出力先がツリー全体になる影響。
5. **段階移行** — kind 廃止・document_tree_links 廃止・統合ロジック追加を一度にやるか、段階的にやるか。

## 対象外（今回の範囲外）

- `item_aliases` の承認 UI 詳細。
- cross-document 統合の LLM プロンプト設計。
- ZIP 展開の挙動（`documents : document_files = 1:N` は維持。ZIP は 1 アップロード = 1 document の出典束）。
