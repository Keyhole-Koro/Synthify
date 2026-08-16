# Paper-Native LLM Interaction

LLM の応答を「チャット欄に流れるテキスト」ではなく、**paper-in-paper のキャンバス操作そのもの**
として扱う構想。回答は paper として空間に配置され、LLM は視界（開閉・focus・pin）を操作でき、
ユーザーの注目度がそのままコンテキスト選択に使われる。

前提となる対話の往復と配信経路は
[../contracts/chat-turn-contract.md](../contracts/chat-turn-contract.md) が定義済み。
本ドキュメントは**その上に何を乗せられるか**を扱い、契約を置き換えるものではない。

## 出発点

現行の設計（[paper-llm-dialogue-child.md](paper-llm-dialogue-child.md)）は、LLM に HTML 文字列を
生成させ、`<a data-paper-id="ID">` を web 側でパースしてクリック可能にする方針だった。

これは **paper-in-paper が既に持っている機能を使っていない**。調査で判明した2点が本構想の根拠になる。

### 根拠1: `ContentNode` は LLM 出力用に設計されている

[core/types.ts:5-19](../../apps/web/vender/paper-in-paper/src/lib/core/types.ts#L5-L19) に構造化
コンテンツ型があり、コメントに `(LLM-friendly, JSON-serializable)` と明記されている。

```ts
export type ContentNode =
  | { type: 'text';       value: string }
  | { type: 'paragraph';  children: ContentNode[] }
  | { type: 'bold';       children: ContentNode[] }
  | { type: 'paper-link'; paperId: PaperId; label: string }
  | { type: 'card';       paperId: PaperId; title: string; description: string }
  | { type: 'section';    title?: string; children: ContentNode[] }
  | { type: 'list';       items: ContentNode[][] }
  | { type: 'table';      headers: string[]; rows: string[][] }
  | { type: 'callout';    children: ContentNode[] }
```

レンダラは実装済みで、`paper-link` / `card` は
[PaperContentNodes.tsx:49-95](../../apps/web/vender/paper-in-paper/src/lib/react/components/PaperContentNodes.tsx#L49-L95)
で `onClick` と `onKeyDown`(Enter) の両方が `onOpen(paperId)` に配線済み、`role="button"` /
`tabIndex` 付き。

つまり **LLM に structured output でこの JSON を生成させれば、参照が最初からクリック可能な UI
要素になる**。HTML 生成もサニタイズもパースも不要になる。`PaperContent` は
`string | ReactNode | ContentNode[]` を受けるので、`ContentNode[]` をそのまま paper の content
にできる。

### 根拠2: Command 語彙がそのまま LLM の行動空間になる

[core/commands.ts:21-47](../../apps/web/vender/paper-in-paper/src/lib/core/commands.ts#L21-L47) の
`Command` union は、LLM の tool schema にほぼそのまま写せる形をしている。

`OPEN_NODE` / `CLOSE_NODE` / `FOCUS_NODE` / `PIN_NODE` / `UNPIN_NODE` / `CREATE_CHILD_NODE` /
`PATCH_NODE` / `MOVE_NODE` / `MERGE_PAPERS` / `DELETE_NODE` …

**LLM が回答テキストを返すのではなく、キャンバスを操作する**という構図が成立する。

## 何が実現できるか

### A. 回答が空間に配置される

「この用語を詳しく」に対してテキストが返るのではなく、その paper の子として新しい paper が開き、
既存の兄弟との位置関係の中に現れる。README の「抽象から具体へ辿る」がそのまま対話に適用され、
**チャット欄という別レイヤーが不要になる**。

### B. LLM が視界を操作する

質問に答える過程で、LLM が `OPEN_NODE` で関連 paper を開き、`FOCUS_NODE` で注目を移し、
`PIN_NODE` で参照中の paper を画面に留める。

「関連箇所を探して」に対して、**テキストで場所を説明するのではなく実際にそこを開いて見せる**。
通常のチャット UI では「〇〇の節を参照してください」としか言えない部分で、これは
paper-in-paper でしか成立しない体験になる。

### C. attention がコンテキスト選択になる

[core/attention.ts](../../apps/web/vender/paper-in-paper/src/lib/core/attention.ts) に減衰付きの
attention モデルが実装済み。`decayAttentionValue` が半減期で減衰し、
[paperCanvasConfig.ts:36-47](../../apps/web/vender/paper-in-paper/src/lib/config/paperCanvasConfig.ts#L36-L47)
に既定値がある。

```
initial: 100, openBonus: 30, focusBonus: 20, labelClickBoost: 50
decayHalfLifeMs: 120_000   // 2分で半減
autoCloseThreshold: 5
```

**今ユーザーが何に注目しているかが既に数値化されている**。現行設計はコンテキスト候補を
「parent + 兄弟 + 祖先」という構造的な近さで選ぶが、`getEffectiveAttention` の高い順に選べば
「さっきまで見ていたもの」が入る。手動チェックボックス UI が不要になる可能性がある。

構造的近さと attention は併用できる（構造で候補を絞り、attention で順位付けする）。

### D. ストリーミングが paper の成長になる

`PATCH_NODE` で content を差し替えられるため、生成中の paper が育っていく様子がそのまま見える。
`contentImportance` / `childMinShares` / `layout`
([types.ts](../../apps/web/vender/paper-in-paper/src/lib/core/types.ts)) でレイアウト比率を
制御できるので、生成中の paper に一時的に広い面積を与え、完了後に縮める演出も可能。

## Command の権限分離（必須）

LLM に Command を発行させる以上、**破壊的操作が混ざりうる**。`DELETE_NODE` や `MOVE_NODE` が
モデルの判断だけで実行されてはいけない。

[contextual-paper-llm-collaboration.md](contextual-paper-llm-collaboration.md) が
preview → Apply を要求しているのと同じ理由で、発行可能な Command を2階層に分ける。

### 自動実行可（読み取り・視界操作のみ）

| Command | 理由 |
|---|---|
| `OPEN_NODE` | 開くだけ。情報を失わない |
| `CLOSE_NODE` | 閉じるだけ。再度開ける |
| `FOCUS_NODE` | 注目移動のみ |
| `PIN_NODE` / `UNPIN_NODE` | レイアウト上の一時的な優先度 |

これらは**元に戻せる**か、**状態を失わない**操作に限られる。ユーザーの明示的な承認なしに
実行してよい。

### Apply 必須（構造変更）

| Command | 理由 |
|---|---|
| `CREATE_CHILD_NODE` | tree に永続的な node が増える |
| `PATCH_NODE` | 既存 content を書き換える |
| `MOVE_NODE` | tree 構造が変わる |
| `MERGE_PAPERS` / `UPSERT_PAPERS` | 複数 node に影響 |
| `DELETE_NODE` | **破壊的。LLM に発行させない** |

`DELETE_NODE` は tool schema から**除外する**。提案する必要がある場合も、削除ではなく
`needs_decision` としてユーザーに投げる。

内部同期用の `__SYNC_*` 系も当然除外する。

## 契約への影響

ここが実務上もっとも重要。**turn doc のペイロード形を先に決める必要がある**。

現行契約は `text: string`（累積回答の全文上書き）。本構想を採るなら:

```
content:  ContentNode[]   // 描画される本体。JSON-serializable
commands: Command[]       // キャンバスに適用する操作（自動実行可のもののみ）
proposals: Command[]      // Apply 待ちの構造変更（preview として表示）
```

`ContentNode[]` は JSON-serializable なので Firestore にそのまま入り、
**「差分ではなく全文上書き」の方針もそのまま使える**（毎回 `content` 配列全体を書く）。

### 順序の推奨

`text: string` で実装してから `ContentNode[]` に移すと、Firestore schema・型生成
（`chat-turn.schema.json` → TS/Go）・worker の出力パース・web の描画がすべてやり直しになる。

**最初から `content: ContentNode[]` にしておく**のが良い。そうすれば:

- A（回答が paper になる）と D（成長するストリーミング）は自然に入る
- B（視界操作）と C（attention）は `commands` フィールドを後から足すだけで乗る

`chat-turn.schema.json` に `content` を最初から入れ、`commands` / `proposals` は
optional として枠だけ用意しておく形を推奨する。

### ContentNode の JSON Schema 表現

`ContentNode` は再帰的な discriminated union なので、
`scripts/generate-firestore-types.mjs` の現行実装（enum / array / プリミティブのみ対応）では
**扱えない**。`$ref` + `oneOf` によるスキーマ表現と、generator 側の再帰型対応が要る。

型生成スクリプトの一般化は
[../contracts/chat-turn-contract.md](../contracts/chat-turn-contract.md) の section 6 で
既に前提作業として挙がっているが、**再帰型対応はそれに加えて必要な追加作業**である。

代替案として、`content` を Firestore 上は不透明な JSON 文字列として持ち、TS 側で
paper-in-paper の `ContentNode` 型にキャストする手もある。generator を触らずに済むが、
schema による検証を失う。初期実装ではこちらでも良い。

## LLM 側の実装

worker の structured output でこれを生成させる。既存の
`GenerateStructured` ([llm/gemini.go:45](../../apps/worker/pkg/worker/llm/gemini.go#L45)) が
使えるが、**ストリーミングとの両立が課題**になる。

- structured output は JSON 全体が揃わないと parse できない。部分的な JSON は不正なため、
  チャンクごとに `ContentNode[]` を作ることができない。
- 対策1: **section 単位で複数回生成**する。1 回の generate で 1 section を返させ、
  section ごとに `content` に追記する。往復は増えるが、成長するストリーミング（D）が成立する。
- 対策2: 初回は `text` でストリーミングし、完了後に構造化する 2 パス。初トークンは速いが
  LLM 呼び出しが 2 倍になる。
- 対策3: ストリーミングを諦め、生成中は「考え中」の paper を出す。実装は最も単純。

初期実装は**対策3**で始め、体験上の必要が出てから対策1に進むのが現実的。
[chat-turn-contract.md](../contracts/chat-turn-contract.md) の
`GenerateTextStream` はこの場合不要になるため、**構造化を採るなら stream の実装を
先送りできる**。

## paper-in-paper 側の変更要否

現時点では**vendored ライブラリの変更は不要**と見ている。

- `ContentNode[]` は `PaperContent` として受理される
- `usePaperDispatch` / `usePaperStoreSelector` は公開エクスポート済み
  ([lib/index.ts](../../apps/web/vender/paper-in-paper/src/lib/index.ts))
- attention 値の読み取りは `usePaperStoreSelector` で `attentionMap` /
  `attentionTimestampMap` を引き、`getEffectiveAttention` を呼べば得られる

ただし `getEffectiveAttention` は現在 `lib/index.ts` から**エクスポートされていない**。
C を実装するなら、この関数（または「attention 順に並んだ paper 一覧」を返すフック）の
エクスポート追加が要る。これは vendored lib への小さな変更になる。

## Open Questions

- **視界操作の煩わしさ**: LLM が勝手に paper を開閉すると、ユーザーの意図した視界が壊れる
  可能性がある。`protectDurationMs`(既定 10 秒) との兼ね合い、および「LLM の操作を
  取り消す」導線が要るか。
- **attention の解釈**: 「さっきまで見ていた」が常に「関連が高い」とは限らない。構造的近さと
  どう重み付けするか。半減期 2 分は対話のコンテキストとしては短い可能性がある。
- **ContentNode の表現力**: コードブロックや画像が型に無い。paper content の主用途が
  リッチ HTML であることとの整合をどう取るか（HTML 版 content と併存させるか）。
- **生成中 paper の扱い**: 失敗した場合に paper を消すか、エラー表示のまま残すか。
- **Apply の粒度**: `proposals` を 1 つずつ Apply させるか、まとめてか。

## 実装順序の目安

```
1. content を ContentNode[] にする（契約確定 → A, D の土台）
2. 対話の往復を動かす（chat-turn-contract の Phase A-E）
3. commands フィールドを足し、自動実行可 Command だけ適用（B）
4. attention をコンテキスト選択に使う（C。lib のエクスポート追加が要る）
5. proposals + preview/Apply（構造変更。contextual 版に合流）
```

1 だけは**後戻りコストが高い**ので先に決める。2 以降は独立して足せる。

## 関連

- [../contracts/chat-turn-contract.md](../contracts/chat-turn-contract.md) — 対話の契約（正本）
- [paper-llm-dialogue-child.md](paper-llm-dialogue-child.md) — 実装計画。HTML 生成前提の部分は
  本構想で置き換わる
- [contextual-paper-llm-collaboration.md](contextual-paper-llm-collaboration.md) — anchor と
  intent の上位構想。preview/Apply モデルは本構想の `proposals` と同じもの
- [paper-in-paper-importance-direction.md](paper-in-paper-importance-direction.md) —
  attention に room を追従させる設計。C と密接に関係する
