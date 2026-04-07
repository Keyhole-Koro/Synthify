# 02. Requirements

## Functional Requirements

### File Upload

- ユーザーは Web UI からファイルをアップロードできること
- アップロードは署名付き URL 経由でフロントエンドから GCS へ直接 PUT する
- 署名付き URL は `GetUploadURL` RPC で発行し、有効期限は15分とする
- 署名付き URL には content-type と最大ファイルサイズ（100MB）を制約として付与する
- アップロードされたファイルは `Cloud Storage` に保存されること
- 保存時に一意な `document_id` を採番すること
- ファイル名、ファイルサイズ、MIME type、保存先 URI、作成日時を記録すること
- アップロード完了後、処理を即時開始すること（バッチ待ちなし）
- zip ファイルは1ファイル = 1 `document_id` として扱うこと
- zip 内の対応外ファイルは無視し、対応ファイルのみ処理すること
- zip のネストは初期スコープに含めない

### ストレージ悪用防止

- workspace のプランに応じたストレージクォータ・ファイルサイズ上限・アップロード数上限を適用する（詳細は [data-model.md](../domain/data-model.md) の `plans` テーブル参照）
- クォータ・上限超過時はアップロードを拒否する
- アップロード後30分以内に処理が開始されないファイルは GCS ライフサイクルルールで自動削除する
- サーバー側でアップロード後の MIME type を検証し、許可外の場合はファイルを削除して `failed` に記録する
- free プランは `summary` モードのみ使用可能とする

### Text Extraction

- PDF、Markdown、TXT、CSV からテキストを抽出できること
- 抽出結果を意味的またはサイズ上の単位で chunk に分割できること
- 各 chunk に `chunk_id` と `chunk_index` を付与すること
- 各 chunk に `source_filename` を付与すること（zip の場合は展開後のファイル名、単ファイルの場合は元ファイル名）
- 可能な範囲でページ番号やオフセットなどの出典位置情報を保持すること

### Node and Edge Extraction by Gemini

- chunk または chunk 群を Gemini に入力できること
- Gemini の返却形式は JSON であること
- 最低限以下を抽出対象とすること
- `concept` / `entity` / `claim` / `evidence` / `counter`
- 各ノードの `level` / `category` / `entity_type` / 説明
- ノード間エッジ
- 出典 chunk
- 同一 document 内で重複する概念は統合可能であること

### Graph Persistence

- 抽出した document、chunk、node、edge を `BigQuery` に保存できること
- canonical 化済み node / edge を探索用グラフとして `Spanner Graph` に同期できること
- エッジには少なくとも `source_node_id`、`target_node_id`、`edge_type` を保持すること
- ノードには少なくとも `label`、`type`、`description` を保持すること
- ノードおよびエッジは出典 chunk を追跡できること

### Graph Retrieval

- document 単位でノード一覧を取得できること
- document 単位でエッジ一覧を取得できること
- node type によるフィルタができること
- 2ノード間の多段経路検索ができること
- 複数 document 横断で canonical node を起点に探索できること

### Visualization

- 階層的なペーパーツリーとしてノードを表示できること（`@keyhole-koro/paper-in-paper`）
- ペーパー内 `summary_html` コンテンツをクリックして関連ペーパーを展開できること
- `hierarchical` エッジはツリー構造として表現し、非階層エッジは `summary_html` 内の `data-paper-id` リンクとして埋め込まれること
- ノードのメタデータパネルから出典 chunk を参照できること
- 経路検索結果をペーパー展開操作として可視化できること

## Non-Functional Requirements

### Scalability

- 初期は MB 級から数百 MB 級を対象とする
- 将来的に GB 級ファイルや大量投入に対応できる構成へ拡張可能であること
- 探索系 API は対話的操作に耐える低レイテンシ構成を持つこと

### Availability and Operability

- 初期構成はサーバレス中心とし、常時運用管理を極小化すること
- 処理失敗時に再処理可能であること
- ログとステータスにより処理進行が把握できること

### Data Quality

- LLM 出力揺れに対応し、再抽出および再統合が可能であること
- 出典追跡により生成結果の監査性を確保すること

### Security

- ストレージバケットは非公開とすること
- API からのみファイルにアクセスできること
- 認証認可は将来的に導入可能な構造とすること
- 個人情報や機密情報を含む場合の保存ポリシーを定義可能であること

### Storage Architecture

- `BigQuery` は document/chunk/node/edge/eval/stats の正本保存と分析に使うこと
- `Spanner Graph` は近傍展開、多段経路検索、対話的 traversal の本番クエリエンジンとして使うこと
- `BigQuery` と `Spanner Graph` の間で canonical graph を同期できること

## Input and Output Specification

### Supported Input Formats

- PDF
- Markdown
- TXT
- CSV

### Future Input Formats

- DOCX
- HTML
- JSON

### Output Format

- フロントとバックエンド間の主たる契約は `Protocol Buffers` とする
- ブラウザとの同期通信には `Connect RPC` を使用する
- ペーパーツリー構築用レスポンスには `nodes`（`summary_html` を含む）と `edges`（`hierarchical` および非階層エッジ）を含めること

```json
{
  "document_id": "doc_001",
  "nodes": [
    {
      "id": "n1",
      "label": "販売戦略",
      "level": 1,
      "category": "concept",
      "description": "販売拡大のための上位方針"
    }
  ],
  "edges": [
    {
      "id": "e1",
      "source": "n1",
      "target": "n2",
      "type": "hierarchical"
    }
  ]
}
```

## Acceptance Criteria

- ユーザーがファイルをアップロードできること
- upload 後に document が記録されること
- テキスト抽出と chunk 保存が行われること
- Gemini からノード・エッジ JSON を取得できること
- BigQuery に document/chunk/node/edge が保存されること
- Spanner Graph に探索用 subgraph が同期されること
- フロントで階層付きノードを含むグラフが表示されること
- ノード詳細から出典 chunk を参照できること
- ノード近傍展開と経路検索が UI から実行できること
