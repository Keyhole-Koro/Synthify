# Workspace content に子paper card を並べる UI 再設計

**Status:** 仕様ドラフト（実装前）
**関連:** README に名前だけ残る `workspace-paper-compact-ui.md`（ファイル欠落）。本ドキュメントが採用方針。

## やりたいこと

ワークスペース paper の `content` の中に、**子paper（document root）を開くための card** を並べる。
card をクリックすると、**今まで通り** paper-in-paper が子paperを横に展開する（room も content も描画／挙動は一切変えない）。

加えて、**ジョブ一覧（最近のジョブ）を常時表示**にする（現状はホバー/ピン時のみ）。

```
ワークスペース paper の content:
┌─ WorkspaceHeader（名前・docs数・状態・ピン）
├─ document root の card 一覧（data-paper-id 付き）  ← クリックで子paper展開
├─ 最近のジョブ                                       ← 常時
└─ ドキュメント追加 UI（ドロップゾーン）
```

## これは新機能ではない（既存パターンの再利用）

paper-in-paper は **content 内に置いた `data-paper-id="<子paperのid>"` 要素のクリックを拾い、その子paperを開く**仕組みを持つ。

- content 内 click を拾い `closest('[data-paper-id]')` で id を取得 → `dispatch({ type: 'OPEN_NODE', parentId, childId })`（[PaperContentFrame.tsx:188-196](../../apps/web/vender/paper-in-paper/src/lib/react/components/PaperContentFrame.tsx#L188-L196)）。
- 開かれた子paperは `layout.childRects` に入り、`PaperNodeFrame` が content の隣に `PaperNode` として描く（[PaperNodeFrame.tsx:149-190](../../apps/web/vender/paper-in-paper/src/lib/react/components/PaperNodeFrame.tsx#L149-L190)）。
- room（`contentShare`/`childShares`）配分・projection・遅延ロードは**従来のまま**。

この card パターンの既存実例:

- **ルートのワークスペース一覧** = `WorkspaceListContent` → `WorkspaceItemList`（[WorkspaceItemList.tsx:19-35](../../apps/web/src/features/paperMap/components/WorkspaceItemList.tsx#L19-L35)）。`data-paper-id={ws.workspaceId}` の `<a>` を content に並べ、クリックで当該ワークスペース paper が開く。
- **ランディングの解説カード** = `PaperLinkCard`（[useLandingPaperMap.tsx:118-159](../../apps/web/src/features/paperMap/hooks/useLandingPaperMap.tsx#L118-L159)）。

→ **本件は「`WorkspaceItemList` がワークスペース card を出すのと全く同じことを、ワークスペース paper の content 内で document root に対してやる」だけ。**

## 設計（採用）

- ワークスペース paper の `content`（[WorkspacePaper.tsx](../../apps/web/src/features/workspaces/paper/WorkspacePaper.tsx)）の中に、`childItems`（document root）の card を `data-paper-id={id}` 付きで並べる。
- クリック → 既存の `OPEN_NODE` 経路で子paperが横に展開（room も content も描画、挙動不変）。
- **`layout` も projection（[useWorkspaceProjection.ts](../../apps/web/src/features/workspaces/useWorkspaceProjection.ts)）も `ExpansionMap` も触らない。**
- `WorkspacePaper` は既に `childItems: { id: string }[]` を受け取っている（[WorkspacePaper.tsx:26](../../apps/web/src/features/workspaces/paper/WorkspacePaper.tsx#L26)）。card にタイトルを出すなら、factory の `childPapers.map((p) => ({ id: p.id }))`（[workspacePaperFactory.tsx:61](../../apps/web/src/features/workspaces/paper/workspacePaperFactory.tsx#L61)）を `{ id, title }` に増やす程度。

### 前提: document : tree は N:1（既に実現済み）

1 ワークスペースに複数 document を入れると、各 document の `document_root`（`kind='document_root'`）が**共通の `workspace_root` の子**として並ぶ（[0008_tree.up.sql](../../db/migrations/0008_tree.up.sql)、`findDocumentRootItemIds` が複数返す [buildTree.ts:45-47](../../apps/web/src/features/tree/buildTree.ts#L45-L47)）。tree 本体は1本（workspace_root は1個）。`document ↔ document_root` は `document_tree_links` の UNIQUE で 1:1。

→ だから content に並べる card は **document ごとに1枚 = 一覧**になる。
（document をまたいだ概念統合 = cross-document tree は別テーマ。`cross_document` フラグは現状未配線、`item_aliases` + `deduplicate_and_merge` は部品のみ。本ドキュメントの対象外。）

### 対象範囲（確定）

- card にするのは **document root（ワークスペース直下の子paper）だけ**。今 `childIds` に渡しているものと同じ単位。
- document root より下のノードは対象外。下層は今まで通り projection 由来の素の paper content（`it.content || <p>desc</p>`、[useWorkspaceProjection.ts:26](../../apps/web/src/features/workspaces/useWorkspaceProjection.ts#L26)）で描かれる。各 paper が自分の子の card を出せば再帰するが、それは今回やらない。

## 検討した代替案（不採用）

- **案: projection の子paper生成をやめ content 内に自前ツリーを描く** → データ構造・遅延ロード・focus を全面再実装し既存設計を上書き。重すぎ。**不採用**。
- **案: `layout` の room 配分を変えて子paperを室展開しない** → やりたいことと無関係（展開挙動は変えたくない）。**不採用**。

## 実装ステップ

### Phase 1 — ジョブ一覧を常に表示（独立・先行）
- [WorkspacePaper.tsx:160-221](../../apps/web/src/features/workspaces/paper/WorkspacePaper.tsx#L160-L221): `WorkspaceJobList` を `isExpanded` ブロックの外に出し、`isPopulated` 時でも常時レンダーする。
- 進捗バー（`WorkspaceJobProgress`）とアップロード欄は従来の hover/pin のままにするか合わせて常時化するかを決める（ジョブ一覧だけ先に常時化が安全）。
- `WorkspaceJobList`（[components/WorkspaceJobList.tsx](../../apps/web/src/features/workspaces/paper/components/WorkspaceJobList.tsx)）は表示位置が変わるだけ。空時 `null` 維持。

### Phase 2 — document root card を content に並べる
- `WorkspacePaper` の content（ヘッダー直下）に `childItems` の card を描く。`WorkspaceItemList` を参考に、`<a data-paper-id={id} className=...>` でタイトル表示。
- click ハンドラは不要（paper-in-paper が `data-paper-id` を拾う）。`onPointerDown`/`onPointerUp` の `stopPropagation` は `WorkspaceItemList` に倣う。
- card にタイトル等を出すため、必要なら factory の `childPapers` を `{ id }` → `{ id, title }` に拡張。
- 新規 `WorkspaceDocumentList`（仮）コンポーネントに切り出すと `WorkspaceItemList` と対になり見通しが良い。

## 未決定事項（実装前に詰める）

1. **card に出す情報** — タイトルのみか、ジョブ状態（処理中/完了）やアイコンも出すか。
2. **配置と常時表示の範囲** — card 一覧をヘッダー直下に常時出すか、`isExpanded` 連動のままにするか（ジョブ一覧と揃える）。
3. **アップロード欄/進捗を常時化するか** — ジョブ一覧だけ常時か、`content` 全体を常時展開（`isExpanded` 廃止）にするか。
4. **空状態** — document root が無いとき（未処理ワークスペース）の card 一覧の見せ方。

## 影響ファイル（見込み）

- `apps/web/src/features/workspaces/paper/WorkspacePaper.tsx`
- `apps/web/src/features/workspaces/paper/workspacePaperFactory.tsx`（`childPapers` に title を足す場合）
- `apps/web/src/features/workspaces/paper/components/WorkspaceJobList.tsx`（位置のみ）
- 新規: `apps/web/src/features/workspaces/paper/components/WorkspaceDocumentList.tsx`（仮）
- 参考（変更なし）: `apps/web/src/features/paperMap/components/WorkspaceItemList.tsx`
