# Paper Map: state と builder のどちらで考えるか

## 背景

`paper-in-paper` の tree を動的に扱うときに、

- `PaperMapBuilder`（mutable builder, `upsert` + `build`）
- React の state と patch

のどちらで考えるべきかが曖昧だった。コードを精査した結果、実態は二択ではなく 3 層に役割分担されているとわかったので、その整理と決定事項をまとめる。

---

## 現状の 3 層構造

| 層 | 担当 | 実装 |
|---|---|---|
| 静的ツリー | `PaperMapBuilder` | `staticPapers.tsx`（ランディング専用） |
| 動的ツリーの組み立て | `Map<PaperId, Paper>` を直接構築 | `useLandingPaperMap` の `useMemo`、`useWorkspaceTree` の `mergeTreeIntoWorkspace` |
| 状態管理・更新 | Command / Reducer | `PaperStoreContext` + `commands.ts` (`UPSERT_PAPERS`, `MERGE_PAPERS`) |

`replaceChildren` / `replaceSubtree` のような独自 subtree patch API を新設するより、既存の Command を拡張する方向の方が実態に合う。

---

## 決定事項

### 1. source of truth は `paperMap` (Map)

`childIds` を正として map 構築時に `parentId` を決定する。`parentId` は lookup 用の補助値であり、直接書き換えない。

### 2. 動的ツリーでは `PaperMapBuilder` を使わない

`PaperMapBuilder` は **静的コンテンツの一回限りの初期化専用**。動的 API データを扱うときは `Map<PaperId, Paper>` を直接組み立てるか、Command を dispatch する。クラス側にも JSDoc で明記済み。

### 3. data state / view state の分離はすでに実装済み

- `PaperViewState` (`types.ts:66-78`) で `paperMap`（data）と `expansionMap` / `focusedNodeId`（view）がフィールドとして独立
- `useDefaultOpenState` / `usePaperCanvasOpenState` が view state 側を担当
- `useWorkspaceTree` が data state 側を担当

この分離は維持する。

### 4. 更新単位は Command 経由（独自 API は新設しない）

- `UPSERT_PAPERS` — 既存を上書き、childIds は置換
- `MERGE_PAPERS` — 既存保持、childIds は union

`mergeTreeIntoWorkspace` (`useWorkspaceTree.tsx:232-250`) がすでにこの方針で動いている。

---

## 未解決の課題

### async 競合管理は不完全

`useWorkspaceTree.tsx:252-268` にあるのは：

- `loadingSubtreeItemsRef` — 同じ item に対する fetch 中の重複防止
- `loadedSubtreeItemsRef` — 一度ロード済みの item を再 fetch しない

「同一 item の重複 fetch 防止」だけで、複数 workspace を素早く切り替えた場合の競合、request 間の順序保証、generation counter / abort signal はいずれも未実装。

実際に問題になるシナリオは限定的（同一 item の subtree fetch では順序が問題にならない）だが、ワークスペース横断の操作が増えると顕在化する可能性がある。

**次に検討する選択肢:**

1. `AbortController` を fetch に渡し、workspace 切り替え時に古い request を中断する
2. parent ごとに request version を持ち、古い version のレスポンスを破棄する
3. 「特に対処しない」と決定して明示的に放置する

別 issue として切り出すのが適切。

### post-update rule（消えた node の expansion / focus 掃除）

subtree が削除されたときに `expansionMap` / `focusedNodeId` に消えた id が残らないか、実装上どこで掃除しているかは未確認。
