# LLM Worker AI 実装メモ

`docs/llm-worker.md` は全体設計仕様として残し、このファイルは **現在の AI 実装の正本** をまとめる。

対象は以下:

- Gemini / AI Studio を使う stage
- prompt / schema の現状
- source file の扱い
- fallback 方針
- 現時点の未実装事項

---

## 1. 現在の方針

LLM worker は、文書をそのまま全文一発処理するのではなく、段階的に tree 候補へ変換する。

現在 LLM を使う主な stage:

- `semantic_chunking`
- `brief_generation`
- `goal_driven_synthesis`
- `html_summary_generation`

LLM が無い、または失敗した場合は、stage ごとに heuristic fallback を持つ。

---

## 2. 使用 API

現在の実装は `google.golang.org/genai` を使い、`BackendGeminiAPI` で Gemini API を呼ぶ。

- API key ベース
- 想定運用は AI Studio / Gemini API
- Vertex AI 前提ではない

client 初期化:

- `worker/pkg/worker/llm/gemini.go`
- `shared/config`

---

## 3. Source File の扱い

### 3.1 基本方針

AI stage には raw text だけでなく original source file も渡す。

そのため、`ContextBundle` と LLM request は `FileURIs` ではなく `SourceFiles []pipeline.SourceFile` を持つ。

`SourceFile` は現在以下を持つ:

- `Filename`
- `URI`
- `MimeType`
- `Content`

`Content` は source file bytes のキャッシュで、同じファイルを複数 stage で二重取得しないために使う。

### 3.2 fetch の流れ

共通 helper:

- `worker/pkg/worker/sourcefiles/fetch.go`

ルール:

- `Content` があれば再取得しない
- `Content` が無ければ `URI` から HTTP GET する
- `MimeType` が空なら response header から補完する

### 3.3 text_extraction との関係

`text_extraction` は source file を fetch して `RawText` を作る。

同じ `SourceFiles` は後続の Gemini stage にも渡される。  
つまり現在は:

- `RawText` = worker 側で抽出したテキスト
- `SourceFiles` = original file bytes

の両方が AI に見える。

### 3.4 ズレの考え方

`RawText` と original file がズレる例:

- PDF の段組み崩れ
- 表構造の喪失
- 見出しの欠落
- 図表キャプションと本文の混線
- OCR 由来の誤読

このため prompt では、構造や見出しや表については original file を優先してよい、という指示を入れている。

---

## 4. Gemini Files API

AI Studio / Gemini API では、単に `file URL` を渡すのではなく Files API upload を使う。

現在の実装:

1. `SourceFile` を fetch
2. `client.Files.Upload(...)`
3. `ACTIVE` になるまで poll
4. `genai.NewPartFromFile(...)` で prompt に追加
5. 生成後に best-effort で delete

実装:

- `worker/pkg/worker/llm/gemini.go`

補足:

- `semantic_chunking`
- `goal_driven_synthesis`

では、`UserPrompt` の text と `SourceFiles` の両方を Gemini に渡す。

---

## 5. Stage ごとの AI 役割

### 5.1 `semantic_chunking`

目的:

- 文書を意味的な chunk に分割する
- 各 chunk に `heading` を付ける

入力:

- `RawText`
- `SourceFiles`

出力:

- `[]Chunk`
- `Outline`

現在の prompt 方針:

- semantic unit 単位で分割
- 見出しは source に寄せる
- 表や箇条書きは近い topic と同居させる
- raw text と original file がズレたら original file を優先する

fallback:

- `splitSections(rawText)` の heuristic

### 5.2 `brief_generation`

目的:

- goal-driven synthesis 用の補助コンテキストを作る

入力:

- outline
- raw text 由来の補助情報

出力:

- `DocumentBrief`
- `SectionBriefs`

失敗時:

- pipeline 継続可能
- heuristic fallback あり

### 5.3 `goal_driven_synthesis`

目的:

- chunk 群から document-level tree を直接合成する

入力:

- `Chunks`
- `Outline`
- `DocumentBrief`
- `SectionBriefs`

出力:

- `SynthesizedItems`
- `SynthesizedEdges`

現在の方針:

- `doc_root` を起点に compact な hierarchy を作る
- item / edge は必ず `source_chunk_ids` を持つ
- 非階層 edge は strongly grounded なものだけ追加する

fallback:

- heading / lead sentence / metric を使った heuristic synthesis

### 5.4 `html_summary_generation`

目的:

- 各 item 用の summary HTML を生成する

方針:

- HTML 断片のみ生成する
- inline CSS は持たせない
- 見た目は frontend 側の共通 CSS に委ねる

---

## 6. Prompt / Schema Versioning

prompt は registry 管理している。

- `worker/pkg/worker/prompts/registry.go`

各 bundle は以下を持つ:

- `PromptName`
- `PromptVersion`
- `SchemaVersion`

system prompt には schema version を明示して渡す。  
これにより、stage ごとの出力 schema を prompt 文面と結びつけて管理する。

---

## 7. Provenance の扱い

provenance は単数 `source_chunk_id` ではなく複数 `source_chunk_ids[]` を使う。

worker 中間表現:

- `RawItem.SourceChunkIDs`
- `SynthesizedItem.SourceChunkIDs`
- `SynthesizedEdge.SourceChunkIDs`

永続化では:

- `item_sources`
- `edge_sources`

を正本にする。

公開 API では item 本体に provenance を埋め込まず、evidence 系で返す方向が正しい。

---

## 8. Fallback 方針

現在の fallback は stage ごとに異なる。

- `semantic_chunking`
  - section split heuristic
- `brief_generation`
  - outline / first sentence ベース
- `goal_driven_synthesis`
  - hierarchy / metric extraction heuristic
- `html_summary_generation`
  - 周辺 description から最低限の HTML を組む

方針としては、

- LLM が失敗しても pipeline 全体を止めない方がよい stage
- LLM 失敗時は空成果より deterministic fallback を優先すべき stage

を分けている。

---

## 9. テスト / Fixture

AI stage では fixture ベースの再現可能テストを使う。

配置:

- `worker/pkg/worker/eval/fixtures/*.json`

現在ある主な fixture:

- `semantic_chunking_pdf_structure.json`
- `goal_driven_customer_acquisition.json`
- `goal_driven_table_metric_grounding.json`

目的:

- raw text と source file のズレを含むケースを固定化する
- LLM response shape の破壊を検知する
- stage logic の validation を壊していないか見る

関連テスト:

- `worker/pkg/worker/stages/semantic_chunking_test.go`
- `worker/pkg/worker/stages/goal_driven_synthesis_test.go`
- `worker/pkg/worker/sourcefiles/fetch_test.go`

---

## 10. 今の制約

まだ弱い点:

- `text_extraction` 自体は PDF 専用の高精度 parser ではない
- Files API upload は都度実行で、再利用 cache はまだ無い
- `brief_generation` / `html_summary_generation` の quality fixture は少ない
- prompt は stage ごとにまだ薄い
- observability は minimal

---

## 11. 次にやる候補

優先順:

1. PDF / table 向け fixture を増やす
2. Files API upload の reuse cache を入れる
3. `brief_generation` と `html_summary_generation` の fixture 拡充
4. stage ごとの prompt をもう少し task-specific にする
5. evidence detail API を整備して provenance を UI で見せる

---

## 12. 実装の正本ファイル

AI 実装を見るときの主な参照先:

- `worker/pkg/worker/llm/gemini.go`
- `worker/pkg/worker/llm/client.go`
- `worker/pkg/worker/context/assembler.go`
- `worker/pkg/worker/context/default.go`
- `worker/pkg/worker/sourcefiles/fetch.go`
- `worker/pkg/worker/stages/text_extraction.go`
- `worker/pkg/worker/stages/semantic_chunking.go`
- `worker/pkg/worker/stages/brief_generation.go`
- `worker/pkg/worker/stages/goal_driven_synthesis.go`
- `worker/pkg/worker/stages/html_summary_generation.go`
- `worker/pkg/worker/eval/fixtures.go`
