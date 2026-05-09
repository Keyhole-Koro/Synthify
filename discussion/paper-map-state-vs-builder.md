# Paper Map: state と builder のどちらで考えるか

## これは何のメモか

`paper-in-paper` の tree を動的に扱うときに、

- `PaperMapBuilder` のような mutable builder で考えるべきか
- React の state と patch で考えるべきか

が少し曖昧になってきたので、まずはやさしく整理するためのメモです。

ここでの目的は「正解を断言すること」ではなく、
**今どこが曖昧で、何を決めると設計しやすくなるか** を見えるようにすることです。

---

## まず、今なぜ迷うのか

今の `PaperMapBuilder` は、paper を順不同で `upsert()` して、最後に `build()` で `parentId` を解決する形です。

たとえば:

```ts
builder
  .upsert({ id: 'auth', ... })
  .upsert({ id: 'workspaces', ... })
  .upsert({ id: 'root', children: ['auth', 'workspaces'], ... })
  .build();
```

これは static な tree を作るには悪くありません。

ただ、今回はそうではなく、

- 最初は node が一部しかない
- 後から API で children が届く
- 展開したときだけ subtree を取得する
- close / reopen / refetch がある

という **遅れて届く tree** を扱っています。

ここで違和感が出ます。

`build()` は「最後に全体を一回組み立てる」発想ですが、実際の UI は
**少しずつ tree が届いて、そのたびに更新される** からです。

---

## `build()` は何をしているのか

今の `build()` はだいたいこうです。

```ts
for (const paper of this.values()) {
  paper.parentId = null;
}

for (const [parentId, childIds] of this.childrenIndex) {
  for (const childId of childIds) {
    const child = this.get(childId);
    if (child) child.parentId = parentId;
  }
}
```

やっていることは単純です。

1. いったん全 paper の `parentId` を消す
2. `childrenIndex` を見て、child の `parentId` を付け直す

つまり、

- 本当に信じているのは `childrenIndex`
- `parentId` は後から作り直す派生値

という設計です。

これは間違いではありません。
ただし、**毎回全件リセットして全件再解決する** のは少し素朴です。

---

## 「もっと賢くできないの？」への答え

できます。

なぜなら、`parentId` が変わるタイミングはそんなに多くないからです。

`parentId` が変わるのは主にこういうときです。

- 新しい child を親にぶら下げた
- 親の children を差し替えた
- node を別の親へ移動した
- subtree を削除した

逆に、こういう更新では `parentId` は変わりません。

- title を変える
- content を変える
- attention を変える

なので、
「全部最後に `build()` し直す」より、
**tree を変えた瞬間に、その差分だけ直す**
方が自然です。

---

## でも本当の問題は、`build()` の賢さだけではない

ここが大事です。

今回の違和感は、
`build()` が遅いとか雑とかだけではありません。

本当は、**動的 API の単位そのもの** が少し合っていない可能性があります。

今の発想はわりとこうです。

- `upsert(paper)`
- `setChildren(parentId, childIds)`

つまり、**1 node ずつ局所的に編集する** 発想です。

でも実際のユースケースはこうです。

- workspace を開いた
- API からその workspace の children がまとめて返ってきた
- 必要なら subtree 全体を置き換えたい

つまり実態は、
**1 node 編集** というより **subtree patch 適用** に近いです。

---

## React Flow はどう考えているか

React Flow は builder を前面に出していません。

基本は 2 パターンです。

1. controlled
   - `nodes` / `edges` を外で持つ
   - change event を受けて state を更新する

2. uncontrolled
   - 内部 state を持つ
   - instance API で add / update する

どちらでも共通しているのは、
**「最終的な state をどう更新するか」** が中心で、
`build()` のような後解決 builder は中心ではないことです。

つまり React Flow 的には、

- `upsert` をいっぱい叩く

より

- `nodes` を新しく作って差し替える
- change を適用する
- subtree 相当のまとまりを更新する

方が自然です。

参考:

- https://reactflow.dev/learn/concepts/adding-interactivity
- https://reactflow.dev/learn/advanced-use/uncontrolled-flow
- https://reactflow.dev/api-reference/utils/apply-node-changes

---

## いま本当に曖昧なのは何か

ここを明確にするとかなり整理しやすくなります。

### 1. source of truth は何か

何を「本当のデータ」として信じるのか。

候補はたとえば:

- `paperMap`
- `childrenIndex`
- `parentId`

今の builder では、
実質的には `childrenIndex` が正で、`parentId` は派生です。

これは悪くありませんが、まず **どれが正か** を言葉で固定した方がよいです。

ここでいう `ground truth` / `source of truth` は、
**同じ情報が複数の場所に入っているときに、最終的にどれを本物として信じるか** という意味です。

たとえば tree 情報が

- `paperMap`
- `childrenIndex`
- `parentId`

に重複して入っているとします。

このとき、どこか1つがズレることがあります。
そのときに

- `childrenIndex` を正として `parentId` を直す
- `parentId` を正として `childrenIndex` を直す

のように、**基準になる側** を先に決めておく必要があります。

`childrenIndex` を正にするのと `parentId` を正にするのの違いはこうです。

- `childrenIndex` / `childIds` を正にする
  - 親から子をたどるのが自然
  - child の順序をそのまま持てる
  - tree UI と相性がよい
- `parentId` を正にする
  - 子から親をたどるのは自然
  - ただし「この親の children は誰か」を毎回集め直す必要がある
  - child の順序管理が弱い

`paper-in-paper` は child の順序が意味を持つので、
基本的には `childIds` 側を正にする方が自然です。

他の案もなくはないですが、いまの議論ではそこまで増やさなくてよいです。
まずは

- `paperMap` の中の `childIds` を正とする
- `parentId` は lookup 用の補助情報にする

くらいで十分です。

`paper node id` は、subtree patch を比較するときのキーとしてそのままで大丈夫です。
影響範囲を考えるときも、基本は `rootId` と descendants の `paperId` 集合で足ります。

`llm worker` の更新が Firestore でフロントへ通知されているかについては、
少なくとも今の repo を見た範囲では **tree 更新のリアルタイム通知は見当たりません**。
Firestore を使っているのは job status 通知側です。

`post-update rule` は難しく聞こえますが、意味は単純です。
**tree を更新したあとに、表示状態を掃除する規則** です。

たとえば subtree patch で node が消えたら、

- `focusedNodeId` がその消えた node を指していたら、親へ戻す
- `expansionMap` に残っている消えた node id は削除する

という後処理をします。

ここでいう `prune` は、「存在しない node を open 状態のまま残さないように取り除く」くらいの意味です。
---

### 2. 更新単位は何か

ここがいちばん大きいです。

候補はたとえば:

- `upsert(paper)`
- `setChildren(parentId, childIds)`
- `replaceChildren(parentId, childPapers)`
- `replaceSubtree(rootId, subtreePapers)`

もし API から遅れて data が届くなら、
後ろの 2 つの方がたぶん自然です。

なぜなら、実際に届くのは「paper 1個」ではなく
**親の直下 children のまとまり** や **subtree のまとまり** だからです。



---

### 3. data state と view state をどう分けるか

これも大事です。

たとえば:

- `paperMap` は data state
- `expansionMap` は view state
- `focusedNodeId` は view state
- sibling share や layout は derived state

という分け方ができます。

ここが曖昧だと、

- loading node を data に入れるのか
- `attention` を data に持つのか layout 側で計算するのか

がぶれます。

> 具体例を書いてくれ

具体例で分けるとこうなります。

- `data state`
  - `paperMap`
  - どの paper が存在するか、親子関係がどうなっているか
- `view state`
  - `expansionMap`
  - `focusedNodeId`
  - fullscreen
  - 今ユーザーがどこを開いて見ているか
- `derived state`
  - layout
  - sibling share
  - effective attention
  - `data state` と `view state` から毎回計算されるもの

たとえるなら、

- `paperMap` は本棚そのもの
- `expansionMap` は今どの棚を開けているか
- layout はその結果、画面上でどう並ぶか

です。

ここを分ける理由は、
「tree の中身が変わった」のか
「見方が変わっただけ」なのか
「表示結果が計算し直されただけ」なのか
を区別しやすくするためです。

---

### 4. async の競合をどうするか

これも見落としやすいですが重要です。

たとえば:

1. A workspace を開く
2. API リクエストが飛ぶ
3. すぐに B workspace を開く
4. B が先に返る
5. 遅れて A が返る

このとき、A の古いレスポンスをどうするかを決めないと、
tree が巻き戻ることがあります。

つまり、async UI では
**何を更新するか** だけでなく
**いつのレスポンスを採用するか**
も設計の一部です。

> 全部をstateに入れる方向性でいいね
> 必要なことを追記しておいて

全部を state に入れる方針でも問題ありません。
むしろ、workspace tree を部分的にしか持たないより自然なことも多いです。

ただし、その場合は次のルールが必要です。

- 古い async レスポンスを捨てる規則
  - 先に開いた A より、あとで開いた B を優先するのか
  - parent ごとに request version を持つのか
- 消えた node に対する `focusedNodeId` / `expansionMap` の掃除
- もう不要になった subtree をいつ state から消すか
  - ずっと保持するのか
  - close 時に消すのか
  - 明示 refresh のときだけ差し替えるのか

つまり、
**全部を state に含めるのはよいが、そのぶん整合性ルールを先に決める必要がある**
ということです。

---

## いまの段階でおすすめの整理

まだ細かい API まで決めなくてよくて、まずは次の 3 つを決めるのがおすすめです。

### A. canonical tree data は `paperMap`

まずは `paperMap` を tree data の source of truth と考える。

`parentId` と `childIds` を両方持つとしても、
「最終的にこの map が正」という立場を取ると整理しやすいです。

---

### B. async 更新の単位は subtree patch

`upsert(paper)` を中心にするのではなく、
まずはこういう操作を中心に考える方が自然です。

- `replaceChildren(parentId, children)`
- `replaceSubtree(rootId, papers)`
- `removeSubtree(rootId)`

これなら API から遅れて届く data と形が合います。

---

### C. `parentId` は正データではなく整合性用の派生値とみなす

もし `childIds` から tree を考えるなら、
`parentId` は lookup を楽にするための補助情報として扱う方が安全です。

つまり、`parentId` を直接いじるというより、
subtree patch を適用した結果として整合するように保つ、という考え方です。

---

## たとえるとどういう話か

この話は、ドキュメントエディタそのものを編集しているというより、
**本棚の並び方をどう更新するか** の話に近いです。

今の builder はこうです。

- 本を1冊ずつ置く
- 最後に「どの棚に属するか」を全部確認し直す

これは最初に一回並べるには悪くありません。

でも今やりたいのはこうです。

- 新しい箱が届いた
- その箱の中には「この棚に入る本の束」が入っている
- その棚の中身をまとめて差し替えたい

このときは「本1冊ずつ `upsert`」より、
**棚ごと入れ替える API** の方が自然です。

---

## いま無理に決めなくていいこと

次のものは、まだ急いで決めなくても大丈夫です。

- `PaperMapBuilder` を最終的に残すかどうか
- `Map` 継承を続けるかどうか
- `build()` を完全に消すかどうか

先に決めるべきなのはそこではなく、

- source of truth
- 更新単位
- async patch の考え方

です。

ここが決まると、builder の形はあとからかなり自然に決まります。

---

## いまの理解として持っておけば十分な一文

> 今の迷いは `build()` の中身の問題というより、
> async に届く tree を、1 node mutation で扱うのか subtree patch で扱うのかがまだ曖昧、という問題。

この理解でひとまず十分です。

---

## 次に決めるとよいこと

次の打ち合わせや設計メモでは、まずこの 3 問だけ決めるのがおすすめです。

1. `paperMap` を canonical data と見なしてよいか
2. 更新単位は `upsert(paper)` ではなく `replaceChildren` / `replaceSubtree` に寄せるか
3. 古い async レスポンスをどう捨てるか

ここが決まると、`PaperMapBuilder` をどうするかはかなり見通しが立ちます。

---

## 決定事項（2026-05-09）

コードの実態（`useLandingPaperMap`）を確認した結果、以下を方針として確定する。

### 1. source of truth は `paperMap` (Map) — 決定

`useLandingPaperMap` はすでに `Map<PaperId, Paper>` を直接組み立てており、これが正しい形。
`childIds` を正として map 構築時に `parentId` を決定する。`parentId` は lookup 用の補助値であり、直接書き換えない。

### 2. 動的ツリーでは `PaperMapBuilder` を使わない — 決定

`PaperMapBuilder` は **静的コンテンツの一回限りの初期化専用**（例：`staticPapers.tsx`）。
動的 API データを扱うときは `Map<PaperId, Paper>` を直接組み立てる。
クラス側にも JSDoc でその旨を明記済み。

### 3. async 競合は fetch hook 側の責任 — 決定

`useLandingPaperMap` は `workspaces` / `workspacePaperGroups` という state を deps に取り、
state が変わるたびに map 全体を `useMemo` で再計算する構造になっている。

これにより古いレスポンスは state に入らない限り map に反映されない。
**「どのレスポンスを採用するか」は fetch hook 側で管理し、paperMap 層は気にしない**という責任分離が成立している。

### 4. 将来の最適化方針（急がなくてよい）

現在は workspace が増えると `useMemo` が map 全体を再計算する。
将来的には `workspaceId` 単位で subtree だけ差し替える形（`replaceChildren(parentId, children)`）に移行すると効率が上がる。
ただし現時点では問題になっていないため、後回しでよい。
