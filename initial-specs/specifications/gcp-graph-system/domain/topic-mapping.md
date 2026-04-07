# 10. Topic Mapping

## Overview

複数ドキュメントにまたがる概念統合を実現するため、上位の `concept` ノードをトピックとして扱い、各ドキュメントのノード群をトピックにマッピングする。マッピングはヒューリスティックによる候補絞り込みと LLM による検証の二段階で行い、LLM 呼び出しコストを抑えながら精度を確保する。

## トピックの定義

- `category=concept` かつ `level in (0, 1)` のノードをトピック候補として扱う
- トピックは単一ドキュメント内で生成されるが、複数ドキュメントから参照される共有概念として扱う
- トピック間の重複は `node_aliases` テーブルでカノニカル化する（詳細は 11-graph-algorithms.md を参照）

## マッピングの処理フロー

```
Step 1: ヒューリスティックで候補を絞り込む
  - chunk/node の embedding とトピック embedding の cosine similarity を計算する
  - 閾値（初期値 0.80）を超えたペアを候補として残す
  - LLM に投げる件数を大幅に削減する

Step 2: LLM で候補を検証する
  - 候補ペアについて「このノード群はこのトピックに属するか」を判定させる
  - 判定理由を必ず返させ、監査可能にする

Step 3: マッピング結果を保存する
  - document_topic_mappings テーブルに保存する
  - method フィールドで heuristic / llm を記録する
```

## ヒューリスティック手法

| 手法 | 精度 | コスト | 用途 |
| --- | --- | --- | --- |
| キーワードマッチ | 低〜中 | ほぼ無料 | 粗い一次絞り込み |
| 編集距離（ラベル類似度） | 中 | ほぼ無料 | 表記揺れの検出 |
| Embedding cosine similarity | 高 | Vertex AI Embeddings 呼び出し | 二次ランキング |

初期実装はキーワードマッチと編集距離で粗く絞り込み、embedding で再ランキングし、最後に LLM で確定する三段階構成を推奨する。

## データモデル

### document_topic_mappings

| Column | Type | Description |
| --- | --- | --- |
| mapping_id | STRING | マッピング識別子 |
| document_id | STRING | ドキュメント識別子 |
| topic_node_id | STRING | トピックノード識別子 |
| confidence | FLOAT64 | 信頼スコア |
| reason | STRING | LLM による判定理由 |
| method | STRING | マッピング手法 |
| created_at | TIMESTAMP | 作成日時 |

#### Method Values

- `keyword` : キーワードマッチによる自動マッピング
- `embedding` : embedding 類似度による自動マッピング
- `llm` : LLM による検証済みマッピング
- `manual` : 人手によるマッピング

## トピックのカノニカル化

ドキュメントをまたいで同一概念が異なるラベルで生成される場合がある（例：「販売戦略」と「セールス戦略」）。このため `node_aliases` テーブルと組み合わせてトピックを統合する。

```
[各 doc の topic candidate ノード生成]
         ↓
[edit distance + embedding で類似トピックを候補化]
         ↓
[高信頼候補は自動承認、境界候補は suggested 登録]
         ↓
[node_aliases に canonical_node_id として登録]
         ↓
[GetGraph 時に canonical ノードに集約して返す]
```

### カノニカル化ルール

document 内の重複統合は Pass 2 を使って Gemini に委ねる。ここでは document 横断の canonical 化のみ扱う。

1. `label` を正規化する（全角/半角のゆらぎ、前後空白、大小文字差を吸収）
2. `label + description` を使って embedding を生成する
3. 正規化後ラベルの編集距離が 2 以下、かつ cosine similarity が 0.97 以上のペアは `node_aliases.merge_status=approved` で自動登録する
4. cosine similarity が 0.88 以上 0.97 未満のペアは `node_aliases.merge_status=suggested` で登録する
5. `suggested` は `dev` ロールがレビューし、承認時に `approved`、却下時に `rejected` へ遷移させる
6. 0.88 未満のペアは候補として保存しない

同名異概念の誤統合を避けるため、embedding とレビュー UI の両方で `label` 単体ではなく `label + description` を扱う。

### Embedding 生成タイミング

- topic candidate の embedding は Pass 2 完了直後に生成する
- 生成処理は document 処理パイプラインの一部として同期的に実行する
- 失敗時は canonical 化のみスキップし、抽出済みノードの保存は継続する

`GraphService.GetGraph` と `NodeService.GetGraphEntityDetail` に `resolve_aliases=true` オプションを追加し、BQ クエリ側で alias JOIN を行う。

## 横断グラフの構造

```
トピック A（canonical concept ノード）
  ├─ doc_001 の level 2-3 ノード群
  ├─ doc_002 の level 2-3 ノード群
  └─ doc_003 の level 2-3 ノード群

トピック B（canonical concept ノード）
  ├─ doc_001 の level 2-3 ノード群
  └─ doc_004 の level 2-3 ノード群
```

## API 拡張

### GraphService への追加

- `GetTopicMap` : トピック一覧と各トピックに紐づくドキュメント・ノード数を返す
- `GetGraph` の `resolve_aliases` パラメータ追加
- `GetGraphEntityDetail` の `resolve_aliases` パラメータ追加

### TopicService（将来）

- `ListTopics` : トピック一覧と統計情報を返す
- `GetTopicDocuments` : トピックに紐づくドキュメント一覧を返す
- `MergeTopics` : 人手によるトピック統合を実行する

## Open Issues

- `approved` の自動閾値（edit distance ≤ 2 かつ cosine ≥ 0.97）が実データで厳しすぎるか
- `suggested` のレビュー負荷をどこまで `dev` ロール運用で吸収できるか
