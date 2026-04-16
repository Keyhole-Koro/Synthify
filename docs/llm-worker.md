# LLM Worker 設計仕様

---

## インフラ構成

### サービス構成

API Service と Worker Service を分離する。全サービス min=0。

```
[Browser]
    ↓ REST / SSE
[API Service]            [Worker Service]
Cloud Run  min=0         Cloud Run  min=0
  - Connect RPC            - PipelineRunner
  - 認証・認可              - Gemini 呼び出し
  - CRUD・検索              - Neon 書き込み
  - SSE（nodes/edges 変更通知）
  - ジョブ状態取得
       ↓                         ↑
  [Cloud Tasks] ─────────────────
       ↓
  [Neon PostgreSQL]  ←── nodes / edges / chunks / workspace / user / jobs
                          pg_bigm で全文検索
```

**min=0 の理由:** 常時リクエストがあるサービスではないため、コールドスタートを許容してコストを最小化する

**リアルタイム更新:** Worker が Neon に書き込むと PostgreSQL LISTEN/NOTIFY が発火し、API Service の SSE エンドポイント（`GET /api/workspaces/{id}/stream`）がフロントに変更イベントを配信する。フロントは受信後に nodes/edges を再フェッチする。

### データストアの役割分担

| データ | ストア | 理由 |
|---|---|---|
| nodes / edges | **Neon PostgreSQL** | AI Agent 書き込み・LISTEN/NOTIFY で即時反映・SQL クエリ |
| document_chunks | Neon PostgreSQL | ソーステキスト参照。検索対象 |
| nodes 全文検索 | Neon PostgreSQL | pg_bigm で日本語 FTS（同一 DB） |
| workspace / user / membership | Neon PostgreSQL | リレーショナルデータ |
| jobs / processing 状態 | Neon PostgreSQL | ジョブ管理 |

すべてのデータを Neon 一本に集約する。Cloud Functions（Firestore trigger）は不要。

### 起動フロー

```
API Service: StartProcessing(docID, wsID)
  → CreateProcessingJob → Neon (status: "queued")
  → Cloud Tasks に enqueue { job_id, document_id, workspace_id, graph_id }
  → job_id を即返す

Worker Service: POST /internal/pipeline
  → Neon から job をロード
  → PipelineContext を構築
  → runner.Run(ctx, pctx)
  → nodes/edges/chunks → Neon（NOTIFY で API Service に通知）
```

ローカル開発では API から Worker に直接 HTTP 呼び出しを行い、Cloud Tasks をバイパスする。

---

## PipelineContext

ステージ間を流れる中央データ構造。実行が進むにつれてフィールドが埋まる。

```go
// worker/pipeline/context.go

type PipelineContext struct {
    // --- ジョブ識別 ---
    JobID       string
    DocumentID  string
    WorkspaceID string
    GraphID     string

    // --- raw_intake ---
    FileURI     string       // GCS URI（単一ファイル）
    SourceFiles []SourceFile // zip 展開後の複数ファイル（zip 以外は len=1）

    // --- text_extraction ---
    RawText string

    // --- semantic_chunking ---
    Chunks  []Chunk
    Outline []string // heading 一覧（後続ステージの Layer 1 コンテキスト）

    // --- brief_generation ---
    DocumentBrief *DocumentBrief // nil でも後続ステージは動く
    SectionBriefs []SectionBrief

    // --- pass1_extraction ---
    Pass1Results map[int]Pass1ChunkResult // key: ChunkIndex

    // --- pass2_synthesis ---
    SynthesizedNodes []SynthesizedNode
    SynthesizedEdges []SynthesizedEdge

    // --- persistence ---
    NodeIDMap map[string]string // local_id → nd_<ULID>

    // SynthesizedNodes[i].SummaryHTML は html_summary_generation が書き込む
}

type SourceFile struct {
    Filename string
    URI      string
    MimeType string
}

type Chunk struct {
    ChunkIndex int
    Heading    string
    Text       string
}

type DocumentBrief struct {
    Topic        string
    Level01Hints []string
    ClaimSummary string
    Entities     []string
    Outline      []string
}

type SectionBrief struct {
    Heading         string
    Topic           string
    NodeCandidates  []string
    ConnectionHints string
}

type Pass1ChunkResult struct {
    ChunkIndex int
    Nodes      []RawNode
}

type RawNode struct {
    LocalID       string
    Label         string
    Category      string
    Level         int
    EntityType    string
    Description   string
    SourceChunkID string
}

type SynthesizedNode struct {
    LocalID       string
    Label         string
    Category      string
    Level         int
    EntityType    string
    Description   string
    SummaryHTML   string
    ParentLocalID string
    ChildLocalIDs []string
}

type SynthesizedEdge struct {
    SourceLocalID string
    TargetLocalID string
    EdgeType      string // "hierarchical" | "supports" | "contradicts" | "measured_by"
}

```

---

## Stage インターフェース

```go
// worker/pipeline/stage.go

type StageName string

const (
    StageRawIntake             StageName = "raw_intake"
    StageNormalization         StageName = "normalization"
    StageTextExtraction        StageName = "text_extraction"
    StageSemanticChunking      StageName = "semantic_chunking"
    StageBriefGeneration       StageName = "brief_generation"
    StagePass1Extraction       StageName = "pass1_extraction"
    StagePass2Synthesis        StageName = "pass2_synthesis"
    StagePersistence           StageName = "persistence"
    StageHTMLSummaryGeneration StageName = "html_summary_generation"
)

type Stage interface {
    Name() StageName
    Run(ctx context.Context, pctx *PipelineContext) error
}
```

### PipelineRunner

```go
// worker/pipeline/runner.go

type PipelineRunner struct {
    stages  []Stage
    jobRepo repository.JobRepository
}

func (r *PipelineRunner) Run(ctx context.Context, pctx *PipelineContext) {
    for _, stage := range r.stages {
        if err := r.jobRepo.SetCurrentStage(pctx.JobID, string(stage.Name())); err != nil {
            // ログのみ。状態更新失敗でパイプラインを止めない
        }
        if err := stage.Run(ctx, pctx); err != nil {
            r.jobRepo.FailJob(pctx.JobID, err.Error())
            return
        }
    }
    r.jobRepo.CompleteJob(pctx.JobID)
}
```

---

## Context Bundle / Assembler

各ステージが Gemini に渡すコンテキストを組み立てる専用の層。
コンテキスト注入ポリシー（後述）をここに集約する。

```go
// worker/context/assembler.go

type ContextBundle struct {
    SystemPrompt  string
    UserPrompt    string
    FileURIs      []string
    TokenEstimate int // Pass2 分割モード判定に使う
}

type Assembler interface {
    ForChunking(pctx *PipelineContext) ContextBundle
    ForBriefGeneration(pctx *PipelineContext) ContextBundle
    ForPass1(pctx *PipelineContext, chunkIdx int) ContextBundle
    ForPass2Normal(pctx *PipelineContext) ContextBundle
    ForPass2Lite(pctx *PipelineContext, sectionIdx int) ContextBundle
    ForPass2Final(pctx *PipelineContext) ContextBundle
    ForHTMLSummary(pctx *PipelineContext, nodeLocalID string) ContextBundle
}
```

`ContextBundle.SystemPrompt` には PromptStore（後述）から読み込んだプロンプトを格納する。

---

## LLM クライアント インターフェース

使用モデル: `gemini-3.0-flash`（環境変数またはconfigで差し替え可能にする）

```go
// worker/llm/client.go

type Client interface {
    GenerateStructured(ctx context.Context, req StructuredRequest) (json.RawMessage, error)
    GenerateText(ctx context.Context, req TextRequest) (string, error)
}

type StructuredRequest struct {
    SystemPrompt string
    UserPrompt   string
    FileURIs     []string
    Schema       any
}

type TextRequest struct {
    SystemPrompt string
    UserPrompt   string
    FileURIs     []string
}
```

### リトライラッパー

```go
// worker/llm/retry.go

type RetryingClient struct {
    inner      Client
    maxRetries int // 通常は 2
}
```

`GenerateStructured` が error を返した場合、JSON repair を試行してから最大 `maxRetries` 回 Gemini を再試行する。

---

## ステージ実行順序

```
raw_intake
  → normalization              (初期実装: 常に skipped)
  → text_extraction
  → semantic_chunking
  → brief_generation           (失敗してもパイプライン継続)
  → pass1_extraction           (チャンク並列)
  → pass2_synthesis            (通常 or 分割モード)
  → persistence                (node_id を確定。nodes/edges/chunks → Neon)
  → html_summary_generation    (ノード並列。確定 node_id を data-paper-id に使う)
    → jobs.status = "completed" (Neon)
```

---

## ステージ別設計

### Stage 1: raw_intake

- GCS から DocumentID に対応するファイルを取得する
- zip の場合は展開し、対応 MIME（PDF / Markdown / TXT / CSV）のファイルのみ `SourceFiles` に追加する
- zip 内の各ファイルは同一 `document_id` に属するものとして扱い、`Filename` で識別する
- 原本は上書きせず、再処理可能な状態を維持する
- `pctx.FileURI`, `pctx.SourceFiles` を書き込む
- LLM 呼び出しなし

### Stage 2: normalization

**初期実装では常に skipped とする。**

インターフェースは予約済み。実装時の流れ:

1. エンコーディング・構造の問題を検出する
2. `problem_pattern` に一致する `approved` ツールが Tool Registry にあれば適用する
3. 一致するツールがなければ Gemini で Python スクリプト案を生成する
4. スクリプトをサンドボックスで dry-run し、Gemini が自動レビューを実行する
   - スコア 0.9 以上 → 自動 `approved`、処理を継続する
   - スコア 0.9 未満 → `documents.status = pending_normalization`、管理者の手動承認を待つ
5. `approved` になった時点で処理を自動再開する
6. 正規化済み成果物を別保存し、原本は不変とする

### Stage 3: text_extraction

`SourceFiles` の各ファイルを種別ごとに処理する:

| MIME | 手法 |
|---|---|
| PDF | Gemini File API（GCS URI を `FileURIs` に渡す）。ページ構造・表・図のキャプションを保持 |
| Markdown / TXT | テキストをそのまま読み込む |
| CSV | `encoding/csv` でパースし行を結合する |
| zip 内複数ファイル | 上記を各ファイルに適用し、`Filename` をセパレータとして結合する |

結果を `pctx.RawText` に書き込む。

### Stage 4: semantic_chunking

**目的:** 固定サイズ分割の代わりに、Gemini がドキュメントの意味的な区切りを判断してチャンクを生成する。

**Gemini への指示方針:**
- セクション・段落・論点の切れ目を認識させる
- 1チャンクは「1つのトピックまたは論点を扱う単位」とする
- チャンクサイズの上限（約 2000 トークン）を設け、超える場合はさらに分割する
- 各チャンクに `heading`（セクション見出し相当）を付与する

**出力スキーマ:**
```json
{
  "chunks": [
    { "chunk_index": 0, "heading": "背景と課題", "text": "..." },
    { "chunk_index": 1, "heading": "施策A: テレアポ強化", "text": "..." }
  ]
}
```

- `pctx.Chunks`, `pctx.Outline`（heading 一覧）を書き込む
- chunks が空の場合はステージ失敗とする

### Stage 4.5: brief_generation

**目的:** 後続ステージの attention を安定させる補助コンテキストを生成する。raw text の代替ではなく、補助レイヤー。

**生成する成果物:**

`DocumentBrief` — 文書全体の高レイヤー要約:
- 文書全体の主題
- level 0〜1 の候補概念
- 主要 claim の要約
- 主要 entity の一覧
- セクション構成の概観

`SectionBrief` — heading ごとの高レイヤー要約:
- セクション主題
- 代表ノード候補
- claim / evidence / counter の有無
- 前後セクションとの接続ヒント
- このセクションで扱う entity / metric の要約

**重要:** brief が raw text と矛盾する場合は raw text を正とする。brief の誤りが疑われても、brief 単体の失敗でパイプラインを止めない。後続ステージは outline と raw text のみで継続できる。

- **失敗してもパイプラインを止めない。** `pctx.DocumentBrief = nil` のまま後続に渡す

### Stage 5: pass1_extraction

**目的:** 各チャンクに対して個別に Gemini を呼び出し、細粒度で全要素を抽出する。

**抽出対象:**

| category | 説明 | 例 |
|---|---|---|
| `concept` | 抽象的・具体的な概念 | 販売戦略、テレアポ施策 |
| `entity` | 実体（組織・人物・数値・日付） | A社、CV率3.2%、2026年Q1 |
| `claim` | 主張・判断・結論 | "SNSの方がROIが高い" |
| `evidence` | 主張を支持する根拠・事例 | "A社でCV率3.2%を達成" |
| `counter` | 主張への反論・留意点 | "テレアポは関係構築に強み" |

**出力スキーマ:**
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

**ID ポリシー:** Pass 1 出力では永続 ID を生成させない。`local_id` のみを使う。Pass 2 入力の組み立て時に `p1_<chunk_index>_<local_id>` に正規化して document 内一意キーにする。

**フィールド要件:**

| フィールド | 必須 | 欠落時の扱い |
|---|---|---|
| `nodes` | 必須 | 配列自体がなければ chunk 全体を再試行 |
| `nodes[].local_id` | 必須 | 欠落した node は破棄 |
| `nodes[].label` | 必須 | 欠落した node は破棄 |
| `nodes[].category` | 必須 | 不正値を含む node は破棄 |
| `nodes[].level` | 必須（0〜3） | 範囲外の node は破棄 |
| `nodes[].source_chunk_id` | 必須 | 欠落した node は破棄 |
| `nodes[].entity_type` | category=entity で必須 | 欠落・不正値 → `"unspecified"` で保存 |
| `nodes[].description` | 任意 | 欠落 → 空文字で保存 |

- 要素単位の不正は repair せず、その要素のみ破棄する
- 破棄後に `nodes[]` が空になった場合は semantic failure → chunk 全体を再試行対象とする

**並列実行:**

```go
type Pass1Stage struct {
    llm         llm.Client
    assembler   context.Assembler
    concurrency int // 同時 Gemini 呼び出し数の上限（例: 5）
}
```

- `errgroup` + セマフォで並列実行する
- 1 chunk でも確定失敗したらステージ全体を失敗とする
- `pctx.Pass1Results` を書き込む

### Stage 6: pass2_synthesis

**目的:** Pass 1 の全チャンク抽出結果をまとめて Gemini に投入し、文書全体の構造を把握させる。

**処理内容:**
1. **重複統合** — 同一概念の表記揺れを統合し、canonical ラベルを決定する
2. **階層割り当て** — 各ノードに level（0〜3）を付与する
3. **クレーム構造の整理** — claim / evidence / counter の論理関係を明確化する
4. **上位概念の補完** — level 0〜1 の抽象概念が不足している場合は補完する
5. **関連リンクの整理** — chunk をまたぐ参照や tree 配置候補を追加する

**階層レベルの定義:**

| level | 名称 | 説明 | 例 |
|---|---|---|---|
| 0 | ドメイン | 文書全体を覆う最上位概念（0〜1件） | 事業戦略 |
| 1 | 概念 | 主要なテーマ・方針 | 販売戦略、マーケティング戦略 |
| 2 | 施策・アクション | 具体的な取り組み | テレアポ施策、SNS施策 |
| 3 | 詳細 | 数値・固有名詞・具体的事実 | CV率3.2%、スクリプト改善 |

**level 割り当てルール:**
- `category=entity` は原則 `level=3`
- `category=claim` / `evidence` / `counter` は内容に応じて `level=2` または `level=3`。文書全体テーマでない限り `level=0` にしない
- 親子関係が明確な場合、子ノードは親ノードより下位の level
- 同一文書内では同等の抽象度のノードに同じ level を割り当てる

**Pass2 モード選択:**

分割モード判定（いずれかを満たす場合）:
- `len(allNodes) > 150`
- `len(pctx.Chunks) > 40`
- `assembler.ForPass2Normal(pctx).TokenEstimate` がモデル上限の 70% 超

**通常モード:** `assembler.ForPass2Normal(pctx)` → Gemini 1 回

**分割モード:**
1. heading でチャンクをセクションに束ねる
2. セクションごとに `assembler.ForPass2Lite(pctx, sectionIdx)` → Pass 2-lite（errgroup で並列可）
   - 重複統合・level 付与・局所リンク整理
3. 各セクションから level 0〜2 の代表ノードとセクション間参照候補を抽出する
4. `assembler.ForPass2Final(pctx)` → 最終 Pass 2（代表ノード群のみ。canonical 化・横断リンク補完）
   - level 3 の詳細ノードは原則セクション内に留め、最終 Pass 2 では再生成しない
5. 1 セクションでも確定失敗したらドキュメント全体を失敗とする

**フィールド要件（Pass 2）:**

| フィールド | 必須 | 欠落時の扱い |
|---|---|---|
| `nodes` | 必須 | 配列自体がなければ document 全体を再試行 |
| `nodes[].local_id` | 必須 | 欠落した node は破棄 |
| `nodes[].label` | 必須 | 欠落した node は破棄 |
| `nodes[].category` | 必須 | 不正値を含む node は破棄 |
| `nodes[].level` | 必須（0〜3） | 範囲外の node は破棄 |
| `nodes[].description` | 任意 | 欠落 → Pass 1 の description を引き継ぐ |
| `nodes[].entity_type` | category=entity で必須 | 欠落・不正値 → `"unspecified"` で保存 |

- 同一 `local_id` が複数回現れた場合は semantic failure → document 全体を再試行
- 破棄後に level 0〜2 の構造ノードが全て消える場合は semantic failure

`pctx.SynthesizedNodes`, `pctx.SynthesizedEdges` を書き込む。

### Stage 7: persistence

- `node_id = nd_<ULID>` を採番して `pctx.NodeIDMap` を構築する
- `pctx.SynthesizedNodes` を Neon の `nodes` テーブルに一括 INSERT する
- `pctx.SynthesizedEdges` を Neon の `edges` テーブルに一括 INSERT する
- `pctx.Chunks` を Neon の `document_chunks` テーブルに一括 INSERT する
- INSERT 完了後に `NOTIFY graph_updated, '{graph_id}'` を発行する（API Service の SSE が受信してフロントに配信）
- `jobs.status` は `processing` のまま維持する（html_summary_generation 完了後に `completed` にする）

### Stage 8: html_summary_generation

**persistence の後に実行する。** `pctx.NodeIDMap` の確定 `nd_<ULID>` を `data-paper-id` に直接使える。

**目的:** ノードごとに個別の Gemini 呼び出しで `summary_html` を生成する。

**入力コンテキスト（ノードごと）:**
- 対象ノードの `label`, `description`, `category`, `level`
- 対象ノードの出典 chunk の原文
- 隣接ノード（親・子・関連リンク候補）の `node_id`（確定済み）, `label`, `category`

**data-paper-id リンクの埋め込み:**
- 内容上関連するノードへの参照を `<a data-paper-id="{node_id}">` 形式で埋め込む
- 関連ノードへの言及が不自然な場合はリンクを省略する
- tree 上で親子として表示されるノードはリンク埋め込みの対象外でよい
- `<a>` タグは `data-paper-id` 属性のみを持つ。`href` や `onclick` を含めない

**許容タグ:** `<table>`, `<thead>`, `<tbody>`, `<tr>`, `<th>`, `<td>`, `<ul>`, `<ol>`, `<li>`, `<p>`, `<h3>`, `<h4>`, `<strong>`, `<em>`, `<a>`

**禁止:** `style` 属性・`<style>`・`<script>`・外部参照タグ

**バリデーション:** 禁止要素を含む場合は `summary_html = null` で保存する。HTML は構文補正しない。制約違反の場合は再生成せず null で保存する。

**失敗扱い:** summary_html 生成失敗はノード単位の部分失敗。document 全体を失敗にしない。フロントは `description` にフォールバックする。

**並列実行:**

```go
type HTMLSummaryStage struct {
    llm         llm.Client
    assembler   context.Assembler
    concurrency int // 同時 Gemini 呼び出し数の上限（例: 10）
}
```

- 完了後に Neon の `nodes.summary_html` を UPDATE する
- 全ノード処理完了後に `jobs.status = "completed"` に更新し、`NOTIFY graph_updated, '{graph_id}'` を発行する

---

## コンテキスト注入ポリシー

入力トークンは安価であるため、各ステージで出力精度を最大化するためにコンテキストを積極的に注入する。

### Layer 0: High-level Brief

- `document_brief`
- 対象 `section_brief`

attention の安定化が目的。raw text より優先しない。brief が raw text と矛盾する場合は raw text を正とする。

### Layer 1: 全ステージ共通（常時注入）

- semantic chunking で生成した文書アウトライン（heading 一覧）

### Layer 2: ステージ別注入

| ステージ | 追加注入するコンテキスト |
|---|---|
| Pass 1（chunk N 処理時） | 全チャンクテキスト + 処理対象 chunk N の明示 |
| Pass 2-lite（分割モード） | 対象セクション内の Pass 1 結果 + section_brief |
| 最終 Pass 2 | document_brief + 各 section_brief の代表要約 |
| HTML サマリ生成 | 対象ノード近傍（親・子・関連）+ 出典 chunk 原文。document_brief は注入しない |

### Layer 3: 横断注入（全ステージ）

- 他ドキュメントの level 0〜1 ノード（topic_mappings から取得）
- トピックマップ（node_aliases の canonical ノード一覧）
- Embedding 類似度上位ノード

Layer 3 は初期から注入する。Embedding 類似度でフィルタリングしてからノイズを除いて渡す。

---

## リトライポリシー

### Gemini 呼び出しのリトライ

- 返却 JSON が不正な場合は JSON repair を 1 回だけ試行する
- JSON repair 後も不正な場合、同一入力に対する Gemini 再試行を最大 2 回まで行う
- LLM 呼び出し自体が失敗した場合も Gemini 再試行を最大 2 回まで行う

### JSON Repair の対象範囲

許容する補正:
- Markdown コードフェンスの除去
- 末尾カンマの除去
- 閉じ括弧・閉じ角括弧の不足補完
- クォート崩れなどの軽微な JSON 構文修正

許容しない補正（semantic error）:
- `level=8` を `3` に補正する
- 不正な `category` 値を既知 enum に寄せる
- 欠落した `label` や `source_chunk_id` を推測補完する

### フォールバック方針

| 状況 | 扱い |
|---|---|
| `description` 欠落 | 空文字で保存 |
| `summary_html` 欠落・バリデーション違反 | null で保存。フロントは `description` にフォールバック |
| `entity_type` 欠落（category=entity） | `"unspecified"` で保存 |
| 構造成立に必須な項目の欠落 | フォールバックせず破棄または再試行 |

---

## エラーハンドリングと Job 状態遷移

```
CreateProcessingJob → status: "queued"
  ↓
PipelineRunner.Run 開始 → status: "running"
  ↓
各ステージ開始 → current_stage: "<stage_name>"
  ↓
成功 → status: "completed"
失敗 → status: "failed", error_message: "<reason>"
```

- ステージのエラーは `fmt.Errorf("stage %s: %w", stage.Name(), err)` でラップして伝播する
- `brief_generation` の失敗は error を返さず nil brief のまま続行する
- `html_summary_generation` のノード単位失敗は error を返さず空 HTML のまま続行する
- 再処理は `StartProcessing` の `force_reprocess=true` で対応する

---

## 自動評価とプロンプト進化

### 設計方針

期待出力（expected output）との一致ではなく、**出力がソーステキストに根拠を持つか（grounding）** を評価軸とする。

LLM の出力は非決定的であり、表現は変わっても内容が正しければよい。
評価すべきは「何が返ってくるか」ではなく「ソースに対して誠実か」。

採点は LLM-as-judge（`gemini-3.0-flash`）が行う。

### アーキテクチャ

```
[EvalDataset]
    ↓
[EvalRunner]  ← 実パイプラインを実行
    ↓
[GroundingJudge]  ← ソーステキストを ground truth として採点
    ↓
[PromptEvolver]   ← 失敗ケースを入力に改善プロンプトを生成
    ↓
[PromptStore]     ← バージョン管理
```

### EvalDataset

期待出力は持たない。ソーステキストのみを ground truth とする。

```go
// worker/eval/dataset.go

type EvalCase struct {
    ID           string
    DocumentText string // これが ground truth
    Description  string // このケースが何をテストするか（人間向けメモ）
}
```

初期は 5〜10 件の手作りケースで十分。

### GroundingJudge

```go
// worker/eval/judge.go

type GroundingJudge interface {
    Judge(ctx context.Context, req JudgeRequest) (GroundingScore, error)
}

type JudgeRequest struct {
    Stage         StageName
    SourceText    string
    ContextBundle context.ContextBundle // Assembler が組み立てたコンテキスト
    Output        json.RawMessage
}

type GroundingScore struct {
    Stage     StageName
    Overall   float64
    Failures  []GroundingFailure
    Reasoning string
}

type GroundingFailure struct {
    NodeLocalID string
    Label       string
    Issue       string // "hallucinated" | "wrong_category" | "wrong_level" | "unsupported_link"
    Detail      string
}
```

**ステージ別の採点観点:**

| ステージ | 採点観点 |
|---|---|
| semantic_chunking | heading がソースのセクション構造を反映しているか |
| pass1_extraction | 各ノードの label/description がソーステキストに根拠を持つか。category/level が内容に合っているか |
| pass2_synthesis | 統合ノードが元チャンクから支持されているか。エッジがソースに根拠を持つか |
| html_summary | data-paper-id リンクの参照先と本文の関連が妥当か。許容タグのみを使っているか |

### PromptEvolver

失敗ケースを入力に、同じコンテキストで改善プロンプトを生成する。

```go
// worker/eval/evolver.go

type PromptEvolver interface {
    Evolve(ctx context.Context, req EvolveRequest) (EvolveResult, error)
}

type EvolveRequest struct {
    Stage         StageName
    CurrentPrompt string
    Failures      []GroundingFailure
    MaxIterations int
}

type EvolveResult struct {
    ImprovedPrompt string
    Reasoning      string
}
```

**進化ループ:**

```
for iter < maxIterations:
    bundle  = assembler.ForXxx(pctx)      // 同じコンテキスト
    output  = llm.GenerateStructured(bundle with currentPrompt)
    score   = judge.Judge(stage, sourceText, bundle, output)
    if score.Overall >= threshold: break
    currentPrompt = evolver.Evolve(stage, currentPrompt, score.Failures)
```

### PromptStore

プロンプトをファイルベースでバージョン管理する。

```
worker/prompts/
  semantic_chunking/v1.txt
  pass1_extraction/v1.txt
  pass2_synthesis/v1.txt
  html_summary/v1.txt
```

`Assembler` は起動時に PromptStore から現行バージョンを読み込む。
評価ループでプロンプトが改善されたら新バージョンとして書き込み、current のポインタを更新する。

---

## ディレクトリ構成

```
backend/
  internal/
    worker/
      pipeline/
        context.go
        stage.go
        runner.go
      context/
        assembler.go    // Assembler インターフェース・ContextBundle
        default.go      // デフォルト実装
      llm/
        client.go
        gemini.go       // GeminiClient（gemini-3.0-flash）
        retry.go
        repair.go
      stages/
        raw_intake.go
        normalization.go
        text_extraction.go
        semantic_chunking.go
        brief_generation.go
        pass1_extraction.go
        pass2_synthesis.go
        persistence.go
        html_summary_generation.go
      eval/
        dataset.go
        judge.go
        evolver.go
        runner.go
      prompts/
        semantic_chunking/v1.txt
        pass1_extraction/v1.txt
        pass2_synthesis/v1.txt
        html_summary/v1.txt
```

---

## 未解決事項

- `normalization` ステージの Tool Registry / Sandbox Runner のインターフェース設計（別仕様で扱う）
- Pass 1 / HTML summary の concurrency 上限の適切な値（Vertex AI クォータに依存）
- `brief_generation` の出力を DB に保存するか（中間成果物として BigQuery or 専用テーブル）
- document 間 canonical 化（`node_aliases` テーブルへの Embedding 類似度マッチング）は別仕様で扱う
- `html_summary_generation` 失敗ノードの再実行手段（ステージ単体での再試行エンドポイント）
- EvalCase のストレージ（ファイル fixtures か DB か）
- PromptStore の current バージョン管理方式（設定ファイル、DB のいずれか）
