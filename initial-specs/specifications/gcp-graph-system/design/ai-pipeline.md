# 08. AI Pipeline

## Overview

AI パイプラインは、原本の保存からグラフ保存までを段階的に処理する。初期段階では単一 document 処理を前提とし、後続で非同期ジョブ化と並列化を行う。

## Pipeline State Model

この仕様では、document 全体の状態と各ステージの状態を分けて扱う。

### DocumentLifecycleState

- `uploaded` : intake 完了、未実行
- `pending_normalization` : 正規化レビュー待ちでブロック中
- `processing` : いずれかのパイプラインステージが進行中
- `completed` : persistence まで完了
- `failed` : いずれかの必須ステージが確定失敗

### PipelineStageState

- `pending` : 未着手
- `running` : 実行中
- `succeeded` : 成功
- `failed` : 再試行上限を超えて失敗
- `skipped` : 条件により不要で実行しなかった

### JobLifecycleState

- `queued` : ジョブ投入済み、未実行
- `running` : 少なくとも 1 つの stage が進行中
- `succeeded` : 必須 stage がすべて成功
- `failed` : いずれかの必須 stage が確定失敗

### Relationship Between States

- `DocumentLifecycleState` は document 全体の外部向け状態
- `JobLifecycleState` は非同期ジョブ 1 件の状態
- `PipelineStageState` はジョブ内の各 `stage_name` の状態
- 通常は `JobLifecycleState.running` の間、document は `DocumentLifecycleState.processing` を取る

### Stage Naming

各ステージはログ・メトリクス・通知で以下の `stage_name` を使う。

- `raw_intake`
- `normalization`
- `text_extraction`
- `semantic_chunking`
- `brief_generation`
- `pass1_extraction`
- `pass2_synthesis`
- `html_summary_generation`
- `persistence`

## Stages

### 1. Raw Intake

- 元ファイルを `Cloud Storage` に保存する
- アップロード完了後、処理を即時開始する
- zip ファイルの場合は展開し、対応フォーマット（PDF / Markdown / TXT / CSV）のファイルのみを処理対象とする
- zip 内の各ファイルは同一 `document_id` に属する chunk として扱い、`source_filename` で識別する
- 原本は上書きせず、再処理可能な状態を維持する

### 2. Normalization（正規化）

問題が検出されない場合はこのステージをスキップする。

- stage_name: `normalization`
- stage state: 問題なしなら `skipped`、自動適用やレビュー待ちなら `running`、承認待ちで document は `pending_normalization` に遷移する

- エンコーディング・構造の問題を検出した場合、`problem_pattern` に一致する `approved` ツールを自動適用する
- 一致するツールがない場合、LLM が新しいスクリプト案を生成する
- スクリプトはサンドボックスで dry-run し、LLM が自動レビューを実行する
- スコア 0.9 以上 → 自動 `approved`、document 処理を継続する
- スコア 0.9 未満 → document を `pending_normalization` に設定し、管理者の手動承認を待つ
- `approved` になった時点で処理を自動再開する
- 正規化済み成果物を別保存し、原本は不変とする

### 3. Text Extraction（ファイル種別ごと）

- stage_name: `text_extraction`

| フォーマット | 手法 |
| --- | --- |
| PDF | **Gemini File API**（PDF をそのまま Gemini に渡してテキスト・構造を抽出）。追加ライブラリ不要、複雑なレイアウトに対応 |
| Markdown / TXT | テキストをそのまま読み込む |
| CSV | Go 標準の `encoding/csv` でパース |
| zip | 展開後に上記を適用（`source_filename` を付与） |

PDF は Gemini File API でページ構造・表・図のキャプションを保持した形で抽出し、そのまま Semantic Chunking に渡す。

---

### 4. Semantic Chunking

- stage_name: `semantic_chunking`

詳細は [extraction-strategy.md](extraction-strategy.md) を参照。

- 正規化済み document からテキストを抽出する
- 固定サイズ分割ではなく、Gemini が意味の切れ目でチャンクを決定する
- 1チャンクは「1つのトピックまたは論点を扱う単位」とする
- チャンクサイズの上限を超える場合はさらに分割する
- 各チャンクに `heading`（セクション見出し相当）を付与する

### 4.5 High-level Brief Generation

- stage_name: `brief_generation`

semantic chunking の直後に `DocumentBrief` と `SectionBrief` を生成する。

#### Named Brief Artifacts

- `DocumentBrief`: 文書全体の主題、level 0〜1 候補、主要 claim / entity、セクション構成を持つ
- `SectionBrief`: heading 単位の主題、代表ノード候補、接続ヒント、entity / metric 要約を持つ

#### Usage Rules

- brief は後続の Pass 1 / Pass 2 に注入する補助コンテキストであり、raw text の代替にはしない
- brief 生成に失敗した場合、outline と raw text のみで後続処理を継続する
- `DocumentBrief` / `SectionBrief` は再生成可能な中間成果物として扱う

### 4. Pass 1 — Fine-grained Extraction（chunk 単位）

- stage_name: `pass1_extraction`

詳細は [extraction-strategy.md](extraction-strategy.md) を参照。

- 各 chunk に対して個別に Gemini を呼び出す
- `concept` / `entity` / `claim` / `evidence` / `counter` を抽出する
- 出典 chunk を各 node に関連付ける
- 不正 JSON には JSON repair を 1 回だけ試行し、なお失敗する場合は同一 chunk の Gemini 再試行を最大 2 回まで行う
- 1 chunk でも確定失敗した場合、その document の処理全体を失敗扱いにする

### 5. Pass 2 — Document-level Synthesis（文書全体統合）

- stage_name: `pass2_synthesis`

詳細は [extraction-strategy.md](extraction-strategy.md) を参照。

- Pass 1 の全チャンク抽出結果をまとめて Gemini に投入する
- `document_brief` / `section_brief` を補助コンテキストとして利用する
- 重複ノードを統合し、canonical ラベルを決定する
- 各ノードに level（0〜3）を付与する
- chunk をまたぐ関連ノード参照を補完する
- claim / evidence / counter のまとまりを node summary と tree 配置に反映する

### 6. HTML Summary Generation

- stage_name: `html_summary_generation`

- ノード抽出・統合が完了した後、別の Gemini 呼び出しでノードごとの HTML サマリを生成する
- Gemini には `<table>`, `<ul>`, `<h3>` などの構造タグのみを出力させ、`style` 属性・`<style>` タグは含めない
- 生成した HTML は `nodes.summary_html` に保存する
- 生成失敗時は null のまま保存し、フロントは `description` にフォールバックする
- フロントは `@keyhole-koro/paper-in-paper` の `PaperContentFrame` を介して `<iframe srcdoc={summary_html}>` で描画し、CSS はアプリ側から注入する
- HTML サマリ生成の失敗は node 単位の部分失敗として扱い、document 全体は失敗にしない

#### data-paper-id リンクの埋め込み

`summary_html` には、そのノードと内容上関連するノードへの `data-paper-id` リンクを埋め込む。

- リンクは `<a data-paper-id="{node_id}">` 形式で記述する（`href` は不要）
- Gemini へのプロンプトに関連ノードの `node_id` と `label` を渡し、本文中で自然な形でリンク化させる
- 関連ノードへの言及が自然でない場合はリンクを無理に埋め込まず省略する
- `data-paper-id` の値は `nodes.node_id`（`nd_*` 形式）を使う
- フロントの `PaperContentFrame` が iframe 内のリンククリックを `postMessage` で検知し、対応ペーパーを `OPEN_NODE` する

例：
```html
<p>この施策は <a data-paper-id="nd_xxx">CV率3.2%</a> を根拠としている。</p>
<p>一方で <a data-paper-id="nd_yyy">テレアポ不要論</a> という反論もある。</p>
```

#### HTML Summary のフィールド要件

- `summary_html`: 任意。欠落または生成失敗時は `null` として保存する
- 許容タグは `<table>`, `<thead>`, `<tbody>`, `<tr>`, `<th>`, `<td>`, `<ul>`, `<ol>`, `<li>`, `<p>`, `<h3>`, `<h4>`, `<strong>`, `<em>`, `<a>` に限定する
- `<a>` タグは `data-paper-id` 属性のみを持ち、`href` や `onclick` を含む場合は semantic invalid とみなす
- `style` 属性、`<style>`, `<script>`, 外部参照タグは semantic invalid とみなし、その node の `summary_html` は破棄する
- HTML は構文補正しない。JSON が正常でも HTML 制約に違反した場合は再生成せず `null` で保存する

### 7. Persistence

- stage_name: `persistence`

- `documents`
- `document_chunks`
- `nodes`（`summary_html` を含む）
- 将来的には `processing_jobs`, `normalization_tools`, `normalization_tool_runs`

## Design Principles

- 原本は不変とする
- LLM には直接データ変換をさせず、可能な限り再利用可能なツールを生成させる
- 変換処理は dry-run と本実行を分離する
- 差分、ログ、失敗理由を追跡可能にする
- ノード化より前に正規化層を置く

## Future Enhancements

- chunk 並列化
- 正規化ツールの自動候補提示
- ツール選択の類似ケース推薦
- 正規化ルールからの半自動テスト生成
