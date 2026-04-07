# 12. Extraction Strategy

## Overview

高精度な知識グラフを構築するため、以下の4つの手法を組み合わせて抽出を行う。

- **セマンティックチャンキング**: LLM が意味の切れ目でドキュメントを分割する
- **多階層ノード**: レベルとカテゴリで表現する
- **2パス抽出**: chunk 単位の細粒度抽出と、文書全体の統合・階層化を分離する
- **クレーム／エビデンス型 + エンティティ型**: 概念だけでなく論理構造と実体を抽出する

---

## Stage 3: Semantic Chunking

固定サイズ分割の代わりに、LLM がドキュメントの意味的な区切りを判断してチャンクを生成する。

### 入力

- 正規化済みドキュメントの全文（または分割可能なセクション単位）

### Gemini への指示方針

- セクション・段落・論点の切れ目を認識させる
- 1チャンクは「1つのトピックまたは論点を扱う単位」とする
- チャンクサイズの上限（例: 2000トークン）を設け、超える場合はさらに分割する

### 出力

```json
{
  "chunks": [
    {
      "chunk_index": 0,
      "heading": "背景と課題",
      "text": "..."
    },
    {
      "chunk_index": 1,
      "heading": "施策A: テレアポ強化",
      "text": "..."
    }
  ]
}
```

---

## Stage 3.5: High-level Brief Generation

semantic chunking の後、抽出の補助コンテキストとして高レイヤー要約を生成する。これは source of truth ではなく、後続ステージの attention を安定させるための補助レイヤーである。

### 生成する成果物

#### document_brief

文書全体の高レイヤー要約。以下を含む。

- 文書全体の主題
- level 0〜1 の候補概念
- 主要 claim の要約
- 主要 entity の一覧
- セクション構成の概観

#### section_brief

`heading` ごとの高レイヤー要約。以下を含む。

- セクション主題
- 代表ノード候補
- claim / evidence / counter の有無
- 前後セクションとの接続ヒント
- このセクションで扱う entity / metric の要約

### 位置づけ

- `document_brief` / `section_brief` は raw text の代替ではない
- 根拠判定は常に source chunk と raw text を優先する
- brief は抽出の補助コンテキストであり、保存済み node / edge より強い権限を持たない

### 保存方針

- `document_brief` は document 単位で 1 件保存する
- `section_brief` は `heading` 単位で保存する
- brief は再処理時に再生成可能な中間成果物として扱う
- 初期実装では BigQuery またはジョブ用中間ストレージに保持し、ユーザー向け API では直接返さない

---

## Stage 4: Pass 1 — Fine-grained Extraction（chunk 単位）

各チャンクに対して Gemini を呼び出し、細粒度で全要素を抽出する。

### 抽出対象

| category | 説明 | 例 |
| --- | --- | --- |
| `concept` | 抽象的・具体的な概念 | 販売戦略、テレアポ施策 |
| `entity` | 実体（組織・人物・数値・日付） | A社、CV率3.2%、2026年Q1 |
| `claim` | 主張・判断・結論 | "SNSの方がROIが高い" |
| `evidence` | 主張を支持する根拠・事例 | "A社でCV率3.2%を達成" |
| `counter` | 主張への反論・留意点 | "テレアポは関係構築に強み" |

### 出力スキーマ

```json
{
  "nodes": [
    {
      "local_id": "n1",
      "label": "テレアポ施策",
      "category": "concept",
      "level": 2,
      "entity_type": null,
      "description": "...",
      "source_chunk_id": "c_001"
    },
    {
      "local_id": "n2",
      "label": "CV率 3.2%",
      "category": "entity",
      "level": 3,
      "entity_type": "metric",
      "description": "...",
      "source_chunk_id": "c_001"
    }
  ]
}
```

### ID ポリシー

- Pass 1 / Pass 2 の LLM 出力では、永続 ID を生成させず `local_id` のみを扱う
- `nodes[].local_id` は同一 document 内で一意であることを求める
- 永続化時にバックエンドが `node_id=nd_<ULID>` を採番する
- Pass 1 の chunk ローカルな `local_id` は、Pass 2 入力の組み立て時に `p1_<chunk_id>_<local_id>` のような document 内一意キーへ正規化してから LLM に渡す
- 将来の canonical 化で生成する正規ノード ID は別識別空間とし、`cn_<ULID>` を用いる
- ラベル、slug、ハッシュ単独値を永続 ID に使わない。表記揺れ、rename、同名別概念で衝突や意味変化が起きるため

### フィールド要件（Pass 1）

| フィールド | 必須 | 欠落時の扱い |
| --- | --- | --- |
| `nodes` | 必須 | 配列自体がなければ chunk 全体を再試行対象とする |
| `nodes[].local_id` | 必須 | 欠落した node は破棄する |
| `nodes[].label` | 必須 | 欠落した node は破棄する |
| `nodes[].category` | 必須 | 不正値を含む node は破棄する |
| `nodes[].level` | 必須 | 0〜3 以外の値を含む node は破棄する |
| `nodes[].source_chunk_id` | 必須 | 欠落した node は破棄する |
| `nodes[].entity_type` | 条件付き必須 | `category=entity` で欠落または不正値なら `unspecified` として保存する |
| `nodes[].description` | 任意 | 欠落時は空として保存する |
- `nodes[]` の配列要素単位で破棄可能な不正値は repair せず、その要素のみ破棄する
- 破棄後に `nodes[]` が空になった場合は chunk 全体を semantic failure とみなし、再試行対象とする

---

## Stage 5: Pass 2 — Document-level Synthesis（文書全体）

Pass 1 の全チャンク抽出結果をまとめて Gemini に投入し、文書全体の構造を把握させる。

### 処理内容

1. **重複統合**: 同一概念の表記揺れを統合し、canonical ラベルを決定する
2. **階層割り当て**: 各ノードに level（0〜3）を付与する
3. **クレーム構造の整理**: claim / evidence / counter の論理関係を明確化する
4. **上位概念の補完**: level 0〜1 の抽象概念が不足している場合は補完する
5. **関連リンクの補完・整理**: chunk をまたぐ参照や tree 配置候補を追加する

### 大きな文書への対応

Pass 1 の全結果がコンテキスト上限に収まらない場合、Pass 2 は二段階で実行する。

1. セクション単位の部分統合を行う
2. 各セクションの代表ノードと補完リンク候補だけを集めて最終統合を行う

分割モードへの切り替え条件は以下のいずれかを満たした場合とする。

- Pass 1 の出力ノード数が 150 を超える
- chunk 数が 40 を超える
- Pass 2 への入力トークン見積もりがモデル上限の 70% を超える

### 分割モードの処理

- semantic chunking の `heading` を使って chunk をセクション単位に束ねる
- セクションごとに Pass 2-lite を実行し、重複統合・level 付与・局所リンク整理を行う
- 各セクションから level 0〜2 の代表ノードとセクション間参照候補を抽出する
- 最終 Pass 2 では代表ノード群のみを対象に、文書全体の canonical 化と横断リンク候補補完を行う
- level 3 の詳細ノードは原則としてセクション内に留め、最終 Pass 2 では再生成しない

### 分割モード時の失敗扱い

- 1 セクションでも Pass 2-lite が確定失敗した場合、その document 全体を `failed` とする
- 最終 Pass 2 のみが失敗した場合も、その document 全体を `failed` とする
- 部分成功の結果は保存しない。再処理時に最初からやり直す

### 階層レベルの定義

| level | 名称 | 説明 | 例 |
| --- | --- | --- | --- |
| 0 | ドメイン | 文書全体を覆う最上位概念 | 事業戦略 |
| 1 | 概念 | 主要なテーマ・方針 | 販売戦略、マーケティング戦略 |
| 2 | 施策・アクション | 具体的な取り組み | テレアポ施策、SNS施策 |
| 3 | 詳細 | 数値・固有名詞・具体的事実 | CV率3.2%、スクリプト改善 |

### level 割り当てルール

- `level` は文書ごとに段数を変えず、常に 0〜3 の4段階で割り当てる
- `level=0` は文書全体を一言で束ねる最上位テーマに限定し、通常は 0 件または 1 件、多くても少数に抑える
- `level=1` は文書の主要なテーマ、方針、章レベルの概念に割り当てる
- `level=2` は具体的な施策、アクション、ワークストリーム、実行手段に割り当てる
- `level=3` は固有名詞、数値、日付、具体的事実、事例、補足ディテールに割り当てる
- `category=entity` は原則として `level=3` に割り当てる
- `category=claim` / `evidence` / `counter` は内容に応じて `level=2` または `level=3` を選び、文書全体テーマに相当しない限り `level=0` にしない
- 親子関係が明確な場合、子ノードは親ノードより下位の `level` に割り当てる
- 同一文書内では、同等の抽象度を持つノードに同じ `level` を割り当て、相対評価ではなく役割ベースで判断する
- 親子配置候補は近い `level` 間を優先し、不要な飛び級を避ける

### 出力スキーマ

Pass 2 でも永続 ID は返させず、Pass 1 で正規化した document 内一意の `local_id` を使い続ける。統合対象ノードは surviving node の `local_id` を残すか、新規補完ノードには新しい `local_id` を付与する。

### フィールド要件（Pass 2）

| フィールド | 必須 | 欠落時の扱い |
| --- | --- | --- |
| `nodes` | 必須 | 配列自体がなければ document 全体を再試行対象とする |
| `nodes[].local_id` | 必須 | 欠落した node は破棄する |
| `nodes[].label` | 必須 | 欠落した node は破棄する |
| `nodes[].category` | 必須 | 不正値を含む node は破棄する |
| `nodes[].level` | 必須 | 0〜3 以外の値を含む node は破棄する |
| `nodes[].description` | 任意 | 欠落時は Pass 1 の description を引き継ぐ |
| `nodes[].entity_type` | 条件付き必須 | `category=entity` で欠落または不正値なら `unspecified` として保存する |
- Pass 2 では document 構造の成立を優先し、要素単位の不正はその node のみ破棄する
- 破棄後に level 0〜2 の構造ノードが全て消える場合は semantic failure とみなし、document 全体を再試行対象とする
- `description` が欠落しても canonical 化や可視化の主処理は継続し、HTML summary 生成で補完機会を持つ
- Pass 2 完了後にバックエンドが surviving `local_id` ごとに `node_id` を採番して永続化する
- 同一 `local_id` の node が複数回現れた場合は semantic failure とみなし、document 全体を再試行対象とする

---

## Extraction Depth

抽出の粒度は `extraction_depth` パラメータで切り替え可能とする。`StartProcessing` RPC で指定する。

| 値 | 説明 | 対象 level |
| --- | --- | --- |
| `full` | 数値・固有名詞レベルまで全て抽出する | 0〜3 |
| `summary` | 施策・アクションまで抽出する（詳細は親ノードの description に含める） | 0〜2 |

- デフォルトは `full`
- Pass 2 で `extraction_depth=summary` の場合、level 3 ノードを親ノードに統合して削除する
- `documents` テーブルに使用した `extraction_depth` を記録し、再処理時に参照可能にする

---

## Context Injection Policy

入力トークンは安価であるため、各ステージで出力精度を最大化するためにコンテキストを積極的に注入する。

### Layer 0: High-level Brief

- `document_brief`
- 対象 `section_brief`

Layer 0 は文書全体とセクション全体の attention を安定させるための補助レイヤーであり、raw text より優先しない。

### Layer 1: 全ステージ共通（常時注入）

- セマンティックチャンキングで生成した文書アウトライン（heading 一覧）

### Layer 2: ステージ別注入

| ステージ | 追加注入するコンテキスト |
| --- | --- |
| Pass 1（chunk N 処理時） | 全チャンクテキスト + 処理対象 chunk N の明示 |
| Pass 2-lite（分割モード時） | 対象セクション内の Pass 1 結果 + section_brief |
| 最終 Pass 2 | document_brief + 各 section_brief の代表要約 |
| HTML サマリ生成 | 対象ノードの隣接ノード（親・子・関連）+ 出典チャンクの原文 |

### Layer 3: 横断注入（全ステージ）

- 他ドキュメントの level 0〜1 ノード（topic_mappings から取得）
- トピックマップ（node_aliases の canonical ノード一覧）
- Embedding 類似度上位ノード（処理対象ノード・チャンクに近いもの上位 N 件）

Layer 3 は初期から注入する。関連性が低いコンテキストはノイズになるため、Embedding 類似度でフィルタリングしてから渡す。

### ステージごとの注入ルール

- Pass 1: `document_brief + section_brief + 対象 chunk raw text`
- Pass 2-lite: `section_brief + 対象 section の Pass 1 結果`
- 最終 Pass 2: `document_brief + 各 section_brief + 代表ノード群`
- HTML サマリ生成: `document_brief` は使わず、対象ノード近傍と出典 chunk を優先する

### 注意点

- brief が raw text と矛盾する場合は raw text を正とする
- brief は level 付与や canonical 化の補助には使うが、source_chunk_id の決定根拠には使わない
- brief の誤りが疑われる場合でも、brief 単体の失敗で document 全体を失敗にはしない。後続ステージは outline と raw text のみで継続可能とする

---

## Retry Policy

- Gemini の返却 JSON が不正な場合は JSON repair を 1 回だけ試行する
- JSON repair 後も不正な場合、同一入力に対する Gemini 再試行を最大 2 回まで行う
- LLM 呼び出し自体が失敗した場合も Gemini 再試行を最大 2 回まで行う
- JSON repair は syntax error のみを対象とし、semantic error は補正しない
- semantic error とは、JSON としては読めるが schema・enum・参照整合性・level 制約を満たさない状態を指す
- `documents.status` を `failed` に更新し、失敗理由をログに記録する
- 再処理は `StartProcessing` の `force_reprocess=true` で対応する
- 評価データ（[evaluation-data.md](../quality/evaluation-data.md)）を使った品質劣化検知で根本原因を特定する

### JSON Repair の対象範囲

- 許容する repair:
  - Markdown code fence の除去
  - 末尾カンマの除去
  - 閉じ括弧・閉じ角括弧の不足補完
  - クォート崩れなどの軽微な JSON 構文修正
- 許容しない repair:
  - `level=8` を `3` に補正する
  - `category=\"foo\"` を既知 enum に寄せる
  - 欠落した `label` や `source` を推測補完する

### フォールバック方針

- 構造成立に必須な項目はフォールバックせず、要素破棄または再試行に回す
- 品質補助項目は保存時にフォールバックを許容する
- 初期実装で許容する主なフォールバックは以下とする:
  - `description` 欠落 → 空文字で保存
  - `summary_html` 欠落 → `null` 保存、フロントは `description` にフォールバック
  - `entity_type` 欠落（`category=entity`）→ `unspecified` で保存

---

## Node Integration Policy（Document 間）

document 内の重複統合は Pass 2 が担う。document 間の統合は以下の順で行う。

1. **Pass 2**: 文書内統合は Gemini が担当し、重複ノードの統合と canonical label の決定を行う
2. **ラベル正規化**: document 間比較では全角/半角、前後空白、大小文字差を吸収する
3. **Embedding 類似度**: Vertex AI Embeddings で `label + description` をベクトル化し、同名異概念の誤統合を避ける
4. **自動承認条件**: 正規化後ラベルの Levenshtein 距離が 2 以下、かつ cosine similarity が 0.97 以上のペアは `node_aliases.merge_status=approved` として自動登録する
5. **要レビュー候補**: cosine similarity が 0.88 以上 0.97 未満のペアは `node_aliases.merge_status=suggested` として登録する
6. **却下条件**: cosine similarity が 0.88 未満のペアは候補を保存しない
7. **承認フロー**: `suggested` は `dev` ロールがレビューし、承認後に canonical_node_id へ読み替え対象とする

embedding は Pass 2 完了直後に生成し、document 処理パイプラインの一部として保存する。

---

### フィールド

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `level` | INT64 | 0〜3（ドメイン→概念→施策→詳細） |
| `category` | STRING | `concept` / `entity` / `claim` / `evidence` / `counter` |
| `entity_type` | STRING | `organization` / `person` / `metric` / `date` / `location`（category=entity のみ） |

---

## Stage 6: HTML Summary Generation と data-paper-id 埋め込み

Pass 2 完了後、ノードごとに個別の Gemini 呼び出しで `summary_html` を生成する。

### 入力コンテキスト

- 対象ノードの `label`, `description`, `category`, `level`
- 対象ノードの出典 chunk の原文
- 隣接ノード（親・子・関連リンク候補ノード）の `node_id`, `label`, `category`

### data-paper-id リンクの埋め込み

内容上関連するノードへの参照を、`summary_html` 内に `<a data-paper-id="{node_id}">` 形式で埋め込む。

- Gemini のプロンプトに関連ノードの `node_id`（`nd_*` 形式）と `label` を渡し、本文中で自然な形でリンク化させる
- 関連ノードへの言及が不自然な場合はリンクを無理に埋め込まず省略する
- `<a>` タグは `data-paper-id` 属性のみを持ち、`href` や `onclick` を含まない
- tree 上で親子として表示されるノードはリンク埋め込みの対象外としてよい

例：
```html
<p>この施策は <a data-paper-id="nd_xxx">CV率3.2%</a> を根拠としている。</p>
<p>一方で <a data-paper-id="nd_yyy">テレアポ不要論</a> という反論もある。</p>
```

### 許容タグと制約

- 許容タグ: `<table>`, `<thead>`, `<tbody>`, `<tr>`, `<th>`, `<td>`, `<ul>`, `<ol>`, `<li>`, `<p>`, `<h3>`, `<h4>`, `<strong>`, `<em>`, `<a>`
- `style` 属性・`<style>`・`<script>`・外部参照タグは semantic invalid とみなし、その node の `summary_html` を `null` で保存する
- HTML は構文補正しない。制約違反の場合は再生成せず `null` で保存する

### 失敗扱い

- `summary_html` 生成失敗はノード単位の部分失敗とし、document 全体を失敗にしない
- 生成失敗・制約違反時は `null` のまま保存し、フロントは `description` にフォールバックする

---

## LLM 呼び出し数の見積もり

1ドキュメントあたりの概算：

| ステージ | 呼び出し数 |
| --- | --- |
| セマンティックチャンキング | 1回 |
| Pass 1（チャンク数に比例） | チャンク数 × 1回（例: 10チャンク → 10回） |
| Pass 2（文書全体統合） | 1回 |
| HTML サマリ生成（ノード数に比例） | ノード数 × 1回（例: 30ノード → 30回） |
| **合計** | **約 42回 / ドキュメント（10チャンク・30ノードの場合）** |

HTML サマリはノード数が多い場合にコストが支配的になるため、バッチ化または並列化を検討する。

---

## Open Issues

- HTML サマリ生成の並列化戦略（Cloud Tasks でノード単位に並列投入するか）
- level 割り当ての一貫性確保（文書間で同じ概念が異なる level になるケースの対処）
