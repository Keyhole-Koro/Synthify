# Paper-in-Paper State Model

`paper-in-paper` の `PaperNode` まわりの状態が `open / closed / collapsed / hidden / focused / drag target` など複数の概念に分かれており、React component 内の条件分岐だけで持ち続けるには複雑になってきている。

この文書は、状態モデルを整理し、どの状態を source of truth にし、どの状態を pure function で導出し、どこまでを React component の責務に残すかを定義するための設計メモ。

## 問題

現状は次の問題がある。

- 状態の種類が増えている
  - `open`
  - `closed`
  - `collapsed`
  - `hidden`
  - `focused`
  - `drag target`
- 状態の由来が分散している
  - `PaperViewState`
  - `layoutMap`
  - `expansionMap`
  - `importanceMap`
  - drag session
- `PaperNode` が「状態判定」と「描画」を両方持っている
- `collapse` のような UI 上の見え方と、`close` のような展開状態が component 条件分岐上で近いレイヤに混ざっている

## 基本方針

次の 3 層に分ける。

1. Canonical state
2. Derived layout facts
3. Derived UI view model

React component は 3 を受け取って描画するだけに寄せる。

## 1. Canonical State

source of truth として保持する状態。

### Tree state

- `paperMap`
- `parentId`
- `childIds`

### Expansion state

- `expansionMap`
  - parent ごとの `openChildIds`

### Focus / interaction state

- `focusedNodeId`
- drag session / insert target

### Temporal weighting state

- `importanceMap`
- `accessMap`
- `protectedUntilMap`

### Local measurement state

- `contentHeightMap`

これらは reducer / store が持つ。

## 2. Derived Layout Facts

layout engine が返す事実。

- `allocatedRect`
- `roomLayout.contentRect`
- `roomLayout.childRects`
- `roomLayout.closedChildIds`
- `hidden?`

重要なのは、ここにはまだ UI 的な意味づけを入れないこと。

例えば `collapsed` は layout fact ではない。  
`contentRect.width = 120` は fact だが、`collapsed = true` は UI interpretation。

## 3. Derived UI View Model

React component に渡す直前に pure function で導出する。

最低限、次の 2 軸に分ける。

```ts
type PaperVisibilityMode =
  | 'normal'
  | 'hidden';

type PaperInteractionMode =
  | 'idle'
  | 'focused'
  | 'drag-target';
```

必要なら将来的に `selected`, `hovered`, `auto-closing-candidate` などを追加する。

### 重要

`closed` は `PaperNode` 自体の visibility mode ではない。

`closed` は expansion state 上の概念であり、

- parent の `openChildIds` に入っていない
- therefore layout 上の `childRects` に出てこない

という意味で扱うべき。

つまり:

- `closed` = tree / expansion 側の状態
- `hidden` = layout / rendering から完全に落とす特殊状態

`collapsed`（content が小さいときに色ブロックで代替表示）は廃止。content が 0 になっても node は normal のまま header だけ表示する。

## 状態の定義

### Open

- parent の `openChildIds` に child が含まれる
- layout 対象になる

### Closed

- parent の `openChildIds` に child が含まれない
- layout 対象から外れる
- subtree expansion も通常は消える

### Hidden

- layout または rendering から完全に落とす
- 現状の実装ではほぼ未使用
- 将来、hidden chain / portal / elision に使う余地はある

### Focused

- `focusedNodeId === nodeId`
- UI accent や interaction priority に影響する

### Drag Target

- drag session 中で insert target に一致する
- outline や drop indicator にのみ影響する

## close / hidden の違い

### close

- expansion から外す
- subtree expansion をクリアする
- layout 対象から外れる

### hidden

- node を render しない
- 現状は概念だけ先にある状態

## 実装責務の再分割

### reducer / store

責務:

- canonical state を持つ
- command を処理する
- layout interpretation は持たない

### layout functions

責務:

- geometry facts を返す
- `hidden` 以外の UI 概念は返さない

### view-model derivation

新規に導入する責務:

- `derivePaperVisibilityMode(...)`
- `derivePaperInteractionMode(...)`
- `derivePaperNodeViewModel(...)`

入力:

- paper
- parentId
- layout entry
- focused state
- drag state

出力:

- visibility mode
- interaction mode
- color / tone input
- index label 必要 여부

### React components

責務:

- view model を受けて描画するだけ

理想的には:

- `PaperNode.tsx`
  - orchestration だけ
- `PaperNodeFrame.tsx`
  - 枠描画
- `PaperNodeContent.tsx`
  - content
- `PaperNodeChildren.tsx`
  - child rendering
## 推奨する pure helper

### 1. 可視性判定

```ts
derivePaperVisibilityMode({
  hidden,
}): 'normal' | 'hidden'
```

### 2. interaction 判定

```ts
derivePaperInteractionMode({
  isFocused,
  isDragTarget,
}): 'idle' | 'focused' | 'drag-target'
```

### 3. node 全体 view model

```ts
derivePaperNodeViewModel(...)
```

## React / Framework レベルでの見直し方針

### 現状

- component 内で条件分岐が増える
- `useLayoutContext`, `usePaperStore`, `useDrag` から直接複数状態を引く
- render 時に複数概念を同時合成している

### 改善

- state selector / derivation を component 外へ出す
- component は render-only に寄せる
- derived state は memoizable な pure function にする

これは React の best practice とも整合する。

- reducer は canonical state だけ持つ
- derived state は selector で作る
- component は props で受ける

## 段階的リファクタ計画

### Phase 2

- `derivePaperNodeViewModel()` の入力を整理し `PaperNode` を orchestrator に縮小

### Phase 3

- 必要なら `hidden` を本当に使う設計へ進める
- room/content サイジングモデルの再設計（`paper-in-paper-layout-model.md` 参照）

## 非目標

- 今すぐ XState などの外部 state machine 導入
- reducer を複数 store に分解する大規模改造
- layout engine の全面書き換え

まずは pure derivation layer を足して、状態の意味を固定するところから始める。
