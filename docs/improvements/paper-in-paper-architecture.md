# Paper-in-Paper アーキテクチャ改善

`paper-in-paper` の状態管理・レイアウト設計に関する改善方針をまとめる。

## 現状の問題

### 1. 状態の二重管理

`expansionMap` / `focusedNodeId` を `page.tsx` の `useState` で持ち、`PaperCanvas` に prop で渡している。
`PaperCanvas` 内部にも同じ状態があり、外側と内側で二重管理になっている。

```
page.tsx (useState)
  └── PaperCanvas (内部 store)
        └── 同じ状態を二箇所で持つ
```

本来 `PaperCanvas` 内部で完結すべきことが外に漏れており、`setExpansionMap` を外から直接叩く設計は内部の command フローを壊している。

### 2. 初期 open state の責務が外にある

`useDefaultOpenState` で初期値を計算して外から注入する形になっているが、「どの branch を最初に開くか」は `PaperCanvas` 内部で完結すべき関心事。

### 3. `attention` の責務過多

現状の `attention` は以下を同時に担っている：

- node 自身の content demand の強調
- subtree を含む room demand の増減
- focus / open による短期的なブースト
- 結果としての視覚的な面積配分

`auth=40`, `workspaces=140` のような値を渡しても、非線形 multiplier・subtree demand 合算・content height・focus/access 更新に引っ張られ、UI が求める表現と内部パラメータの距離が遠い。

---

## 目標

- `expansionMap` / `focusedNodeId` を `PaperCanvas` 内部の command で管理する
- 初期 open state の決定ロジックを `PaperCanvas` 内部に閉じる
- `attention` を content emphasis に絞り、room 配分は別概念で扱う
- 呼び出し側は `defaultOpenState` を渡すだけでよい形にする

## 非目標

- `attention` モデルの完全廃止
- すべての parent node に複雑な custom layout policy をすぐ導入すること
- `paperMap` 自体に view policy を埋め込むこと

---

## 改善方針

### 1. 状態管理を command に統一する

外から `setExpansionMap` を直接叩くのをやめ、すべて command 経由にする。

```ts
// Before
setExpansionMap((prev) => { ... });
setFocusedItemId(workspaceId);

// After
dispatch({ type: 'OPEN_NODE', nodeId: workspaceId });
dispatch({ type: 'FOCUS_NODE', nodeId: workspaceId });
```

`onExpansionMapChange` / `onFocusedNodeIdChange` は保存専用のコールバックとして残す。

### 2. 初期 open state を PaperCanvas 内部に閉じる

```ts
<PaperCanvas
  paperMap={paperMap}
  rootId={ROOT_ID}
  defaultOpenState={computeDefaultOpenState({ user, workspaces })}
  onExpansionMapChange={saveExpansionMap}
  onFocusedNodeIdChange={saveFocusedItemId}
/>
```

- `PaperCanvas` が起動時に persisted state → `defaultOpenState` → ライブラリ初期値の優先順位で初期化する
- `useDefaultOpenState` hook は不要になる

### 3. Sibling Share Rule

`attention` から room 配分の責務を分離し、siblings 内で閉じた配分ルールを別概念として持つ。

```ts
type SiblingShareRule =
  | { type: 'natural' }
  | { type: 'fixed'; shares: Record<string, number> }  // 合計1.0でなくても内部で正規化
  | { type: 'focused'; focusedId: string; focusedShare: number }; // 残りは等分
```

`computeSiblingShareRule` は純粋関数として実装し、`computeNodeLayout` の share 計算直前で適用する：

```
1. attention から content demand を出す
2. subtree を含めた room demand を出す
3. demand から natural share を作る
4. computeSiblingShareRule(parentId, ...) を呼ぶ
5. rule があれば siblings 内 share を補正する
6. 補正後 share を使って rect を pack する
```

重要なのは `attention` を消すのではなく、最後の room 配分だけを別ルールで補正すること。

### 4. useSiblingShare hook（高レベル API）

`computeSiblingShareRule` は内部実装として残しつつ、呼び出し側は hook 一本で済むようにする。

```ts
const { shareOf } = useSiblingShare(rootPaper, { whenFocused: 0.8 });
const authShare = shareOf(authPaper);       // 0.8 (focused 時)
const wsShare   = shareOf(workspacesPaper); // 0.2 (残り)
```

- 引数は id 文字列ではなく `PaperDef` オブジェクト。typo がなくなり、参照が切れたら型エラーになる。
- `whenFocused` が指定されていれば `focused` rule、なければ `natural` を自動選択。
- `childIds` / `focusedNodeId` / `expansionMap` / `paperMap` は hook 内部で context から取得。

**現状の制約:** `useSiblingShare` は `PaperStoreProvider` 内部でしか呼べない。
`page.tsx` は `PaperCanvas` の外で attention を計算しているため、現時点では `useSiblingShare` の置き所がない。
方針 1・2 が完了して状態管理が内部に閉じてから本格導入する。

### 5. PaperMapBuilder

`definePaper` / `definePapers` の代わりに、`Map` を継承した builder class で paper を管理する。

```ts
const papers = new PaperMapBuilder()
  .upsert({ id: 'auth', title: 'アカウント', hue: 250, content: null })
  .upsert({ id: 'workspaces', title: 'ワークスペース', hue: 200, content: null })
  .upsert({ id: 'root', title: 'トップ', hue: 230, content: <section>...</section>, children: ['auth', 'workspaces'] })
  .build(); // parentId を全解決

export const STATIC_PAPERS = [...papers.values()];
```

- `children` は id 文字列の配列。`build()` で `parentId` を全解決する。
- `Map<PaperId, Paper>` を継承するので既存の `PaperMap` 型と互換。
- 動的な paper 追加（`useLandingPaperMap` 等）にも `upsert()` で自然に対応できる。

---

## 段階的導入案

### Phase 1（現在）

- `computeSiblingShareRule` / `useSiblingShare` を実装済み（未結線）
- `useDefaultOpenState` で初期 open state を外から注入（暫定）
- `definePaper` / `definePapers` で静的 paper を定義

### Phase 2

- `PaperCanvas` に `defaultOpenState` prop を追加
- 状態初期化を内部に移す
- `useDefaultOpenState` を廃止

### Phase 3

- command を整理して外からの直接 `setExpansionMap` を廃止
- `useSiblingShare` を `PaperCanvas` 内部から呼び出せるようにする

### Phase 4

- `PaperMapBuilder` を実装して `definePaper` / `definePapers` を置き換える
- `attention` を content emphasis に絞り、`SiblingShareRule` で room 配分を担う

---

## 未決定事項

- `SiblingShareRule` は natural share の上書きか、下限保証か
- `focusedShare` が content demand を完全に無視してよいか
- root 直下以外にどこまで custom rule を許すか
- `computeDefaultOpenState` に `user` / `workspaces` 以外の入力が必要か
- persisted state にアカウント ID を紐付けて不一致時は破棄するか（[auth-open-state-behavior.md](auth-open-state-behavior.md) 参照）

## 関連ドキュメント

- [auth-open-state-behavior.md](auth-open-state-behavior.md)
