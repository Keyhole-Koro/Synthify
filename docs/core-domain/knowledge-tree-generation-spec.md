# Knowledge Tree Knowledge Tree Generation 仕様

## 何を作るか

ドキュメントから paper-in-paper で読めるナレッジツリーを生成する。

paper-in-paper の UX は「リンクをクリックすると子ペーパーが親の中に展開される」インタラクション。
つまり **各アイテムは独立して読める1つの概念** であり、**親は子へのリンクを本文中に含む** ことで探索の動線を作る。

---

## アイテムの設計原則

### タイトル（label）

- 名詞句。「〜について」「〜の説明」はつけない
- 検索ワードになる言葉を選ぶ
- 15字以内が理想

例：`✅ 用量計算の原則` `❌ 用量計算の原則について`

### 本文（summary_html）

- 1〜3段落の HTML
- 段落は `<p>` で囲む
- キーワードは `<strong>` でマーク
- 子アイテムへのリンクは `<a data-paper-id="{child_id}">テキスト</a>` で埋め込む
  → paper-in-paper がこのリンクをクリック可能なチップとして描画する
- テーブルは `<table>` でそのまま書いてよい

### 概要（description）

- 1文。親のチップカードに表示される
- 「〜を扱う」「〜を説明する」で終わらせる

### 階層

- 深さに制限はない。LLM が文書構造に合わせて判断する
- ただし、各アイテムは **単体で読んで意味が通じる** こと
- 親の内容を読まないと理解できないアイテムは粒度が細かすぎる

---

## デフォルトプロンプト

アップロード時にユーザーが何も指定しなかった場合に適用する。
`generate_knowledge_tree` のシステムプロンプトに追加注入する。

```
You are building a knowledge tree for paper-in-paper, a UI where each item
opens inside its parent as an expandable paper.

Authoring rules:
- label: a concise noun phrase (≤ 15 chars preferred). No trailing "について".
- description: one sentence summarizing what this item covers.
- summary_html: 1-3 <p> paragraphs. Bold key terms with <strong>.
  Link to child items with <a data-paper-id="{local_id}">term</a> so readers
  can expand them inline. Use <table> for tabular data.
- Each item must be self-contained: a reader should understand it without
  having read the parent.
- Depth is unlimited. Let the document structure dictate the hierarchy.
  Do not artificially flatten or cap levels.
```

---

## ユーザー指定プロンプト

アップロード時に任意で入力できる追加指示。デフォルトプロンプトの後ろに連結する。
**プリセット選択 + 自由入力の組み合わせ**で提供する(未決事項だった「自由入力だけか、
プリセットを選べるようにするか」への回答)。プリセットは chip として複数選択でき、選ぶと
対応する指示文が `knowledge_tree_prompt` に追記される。追記後もテキストは自由に編集できる
── プリセットは「よく使う指示の下書き」であって、別軸の構造化パラメータにはしない。

**プリセット(chip として提示):**

| プリセット | 挿入される指示 | 効果 |
|---|---|---|
| 散文で | `箇条書きより散文で説明してください` | `<ul>` を減らし段落を増やす |
| 用語解説重視 | `用語解説を重視して、各アイテムに定義を必ず含めてください` | 専門用語アイテムを増やす |
| 章構成を維持 | `章構成を忠実に反映してください` | ドキュメントの見出し階層をそのまま使う |
| 専門用語のまま | `読者は医療従事者です。略語は展開せず使ってください` | 専門用語をそのまま残す |

プリセット文言はドキュメントの言語ではなく **UI の表示言語** に合わせて出し分ける(生成結果に
影響するのは連結後のプロンプト文字列そのものであり、プリセットの選択 UI 自体は多言語対応が要る)。

---

## 実装方針

### 1. データモデル

`documents` テーブルに `knowledge_tree_prompt TEXT` カラムを追加。
アップロード時に保存し、ジョブ開始時に `ProcessDocument` へ渡す。

### 2. プロンプト注入

`Orchestrator.ProcessDocument` に `synthPrompt string` 引数を追加。
`BeforeModelCallback` で Working Memory に続けて注入する。

```go
// BeforeModelCallback 内
systemInstruction := existingInstruction +
    "\n\n" + workingMemory +
    "\n\n## Knowledge Tree Generation Style Guide\n" + defaultPrompt +
    "\n\n" + userPrompt  // userPromptが空なら省略
```

全ツール（brief・knowledge tree generation・critique）が同じシステムプロンプトを受け取るため、
スタイルが一貫する。knowledge tree generation の `instruction` フィールドは引き続き
「このチャンクに特化した追加指示」用として残す。

### 3. UI

アップロードフォームに、プリセット chip の行 + 任意テキストエリアを追加する。
chip をクリックすると対応する指示文がテキストエリアに追記される(複数選択可、重複追記は
避ける)。テキストエリアは常に編集可能で、chip を使わず自由入力のみでもよい。
プレースホルダーに「例：用語解説を重視してください / 箇条書きより散文で」など。

このフォームは [local-llm-provider.md](../improvements/local-llm-provider.md) の
Phase 5 が追加するモデル選択 UI と同じアップロードフォームに並ぶが、データモデルは分離する
(`knowledge_tree_prompt` と `model_selection` は別カラム、別の関心事)。

---

## 解決済みの未決事項

- **ユーザープロンプトのプリセット選択 UI**: プリセット chip + 自由入力の組み合わせにする。
  詳細は「ユーザー指定プロンプト」を参照。

## 未決事項

- `knowledge_tree_prompt` カラムの文字数上限(とりあえず 2000 字程度を想定。chip 追記分もこの
  上限に含める)
- デフォルトプロンプトの多言語対応(日本語ドキュメントには日本語で生成させるか)
- プリセットの一覧をこの4つで固定するか、利用ログを見て追加/削除する運用にするか
