# Roadmap

## Extension Roadmap

### Near-Term Extensions

- `Cloud Tasks` による非同期ジョブ化
- 大きなファイルの chunk 並列処理
- ノード重複統合ルールの強化
- フロントでの検索・フィルタ・折りたたみ
- `buf` を使った proto 管理とコード生成パイプライン
- 正規化ツールのレビュー、承認、再利用フロー

### Mid-Term Extensions

- `Pub/Sub` による大量投入
- `Cloud Run Jobs` による再処理バッチ
- `Memorystore` によるレスポンスキャッシュ
- `BigQuery` の scheduled query による夜間再集計

### Advanced Extensions

- `Vertex AI Embeddings` または類似技術による類似ノード探索
- 複数 document 横断の概念統合
- `Spanner Graph` への移行または併用
- 高度なグラフ探索 API の追加

## Resolved Decisions

- PDF 抽出: Gemini File API を使用する
- 正規化ツール approval: LLM スコア ≥ 0.9 で自動承認、< 0.9 で人間レビュー（Discord 通知）
- ファイルアップロード: フロントから GCS へ署名付き URL で直接アップロードする
- 認証: Firebase Auth + Google OAuth を採用（認証なし MVP フェーズなし）
- ノード統合: Pass 2 の文書内統合は Gemini に委ね、文書横断 canonical 化は `edit distance ≤ 2 && cosine ≥ 0.97` を自動 `approved`、`cosine ≥ 0.88` を `suggested` とする
- フロント可視化: `React Flow` を採用する
- `/dev/stats` の表示制御: Firebase Auth カスタムクレーム `role: "dev"` で制御する
- モバイル対応: 初期スコープ外とする

## Open Issues

- Gemini の出力スキーマ詳細（field 必須/任意、enum の許容値、repair 対象の範囲）
- 差分 graph 再計算の伝播範囲を 2-hop から広げる必要があるか
