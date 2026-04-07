# 02. Requirements (Synthify)

## Functional Requirements

### File Upload

- ユーザーは Web UI からファイルをアップロードできること
- アップロードは署名付き URL 経由でフロントエンドから GCS へ直接 PUT する
- 署名付き URL は `GetUploadURL` RPC で発行し、有効期限は15分とする
- 署名付き URL には content-type と最大ファイルサイズ（100MB）を制約として付与する
- アップロードされたファイルは `Cloud Storage` に保存されること
- 保存時に一意な `document_id` を採番すること
- ファイル名、ファイルサイズ、MIME type、保存先 URI、作成日時を記録すること
- アップロード完了後、処理を即時開始すること

### Text Extraction

- PDF、Markdown、TXT、CSV からテキストを抽出できること
- 抽出結果を意味的またはサイズ上の単位で chunk に分割できること
- 各 chunk に `chunk_id` と `chunk_index` を付与すること

### Paper Tree Extraction by Gemini

- chunk または chunk 群を Gemini に入力できること
- Gemini の返却形式は JSON であること
- 最低限以下を抽出対象とすること
  - `paper`: 概念や実体を表す単位
  - `parentId`: 親 Paper の ID
  - `title`: Paper のタイトル
  - `description`: Paper の概要（ホバー時に表示）
  - `content_html`: Paper のメインコンテンツ（iframe 内に表示）
  - `child_ids`: 子 Paper の ID 一覧
- `content_html` 内には、子 Paper を展開するためのリンク `<a data-paper-id="xxxx">...</a>` を埋め込むこと
- 抽出された構造は循環のない木構造（Root は一つ）であること

### Paper Persistence

- 抽出した document、chunk、paper を `BigQuery` に保存できること
- `papers` テーブルには `id`, `parent_id`, `title`, `description`, `content_html`, `child_ids` を保持すること
- 各 Paper は出典となる chunk ID を保持し、追跡可能であること

### Paper Retrieval

- document 単位で Paper Tree のメタデータを取得できること
- 特定の Paper ID を指定して、その Paper の `content_html` および直近の子 Paper 情報を取得できること
- ユーザーの操作（展開）に合わせて、必要に応じて下位の Paper 情報を遅延読み込みできること

### Visualization & Interaction (paper-in-paper)

- `paper-in-paper` ライブラリを用いて「部屋（Room）」をレンダリングすること
- 各 Paper のコンテンツは隔離された iframe 内に表示されること
- iframe 内のリンククリックで、親の部屋内で該当する子 Paper を展開すること
- ユーザーが Paper をドラッグして Grid 上の配置を変更できること
- 各 Paper は「重要度（Importance）」の状態を持ち、時間経過や非アクティブ状態で減衰すること
- 部屋内のスペースが不足した場合、重要度の低い Paper を自動的に閉じる（Shrink）こと
- パンくずリストにより、Root から現在フォーカスしている Paper までのパスを表示・ナビゲートできること

## Non-Functional Requirements

### Scalability

- 初期は MB 級から数百 MB 級を対象とする
- 深いネスト構造を持つドキュメントでも、表示中の部屋のみを優先して描画する構成にすること

### Data Quality

- LLM が生成する `content_html` 内の `data-paper-id` と `child_ids` の整合性を確保すること（リンク先の Paper が必ず `child_ids` に存在すること）

## Input and Output Specification

### Supported Input Formats

- PDF / Markdown / TXT / CSV

### Output Format

- フロントとバックエンド間の主たる契約は `Protocol Buffers` とする
- `GetPaperTree` RPC は Root Paper から始まる再帰的構造、またはフラットな配列を返す

```json
{
  "document_id": "doc_001",
  "papers": [
    {
      "id": "p1",
      "parent_id": null,
      "title": "量子コンピュータの基礎",
      "description": "量子力学の原理を利用した計算機",
      "content_html": "<p>量子コンピュータは<a data-paper-id=\"p2\">量子ビット</a>を基底として...</p>",
      "child_ids": ["p2", "p3"]
    },
    {
      "id": "p2",
      "parent_id": "p1",
      "title": "量子ビット",
      "description": "0と1の重ね合わせ状態を持つ情報の最小単位",
      "content_html": "<p>量子ビット（Qubit）は、重ね合わせと量子もつれを利用して...</p>",
      "child_ids": []
    }
  ]
}
```

## Acceptance Criteria

- ユーザーがファイルをアップロードできること
- Gemini から再帰的な Paper 構造（HTML コンテンツ含む）を取得できること
- フロントエンドで「部屋」の中に Paper が表示され、クリックで子が開くこと
- ドラッグによる配置変更が保持されること
- 未使用の Paper が自動的に縮小されること
