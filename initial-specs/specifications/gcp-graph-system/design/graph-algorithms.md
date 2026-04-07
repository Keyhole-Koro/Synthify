# 11. Graph Algorithms

## Overview

トピックマッピングと横断ノード集合の構築が完了した後、グラフアルゴリズムを適用して概念の重要度・クラスタ・関係経路を分析する。`BigQuery` は正本保存・分析・バッチ計算結果保持に使い、`Spanner Graph` は対話的な近傍探索と多段経路検索に使う。アルゴリズム処理は Cloud Run Jobs でバッチ実行し、結果を BigQuery に保存してフロントエンドの可視化に利用する。ここでいう「graph」は edge API を意味せず、node 間の探索用 adjacency を持つ内部表現を指す。

## 対象アルゴリズム

| アルゴリズム | 用途 | 優先度 |
| --- | --- | --- |
| PageRank / 中心性分析 | 重要概念の自動ランキング | 高 |
| コミュニティ検出 | トピッククラスタの可視化 | 高 |
| 最短経路 | 概念間の関係経路の探索 | 中 |
| 類似ノード推薦 | 関連概念のサジェスト | 中 |
| 到達可能性分析 | 影響範囲の把握 | 低 |

## 処理アーキテクチャ

```
[BigQuery: nodes]
         ↓ 処理完了トリガー or 定期実行
[Cloud Run Jobs: graph algo worker]
  - NetworkX または igraph で計算
  - PageRank スコア / コミュニティ ID / 中心性を計算
         ↓
[BigQuery: node_scores テーブル]
  - node_id, algo_type, score, computed_at
```

アルゴリズム処理をメインの API サーバから分離することで、重い計算がリクエスト処理に影響しない。

## データモデル

### node_scores

| Column | Type | Description |
| --- | --- | --- |
| node_id | STRING | ノード識別子 |
| algo_type | STRING | アルゴリズム種別 |
| score | FLOAT64 | スコア値 |
| metadata | JSON | アルゴリズム固有の追加情報 |
| computed_at | TIMESTAMP | 計算日時 |

#### algo_type Values

- `pagerank` : PageRank スコア
- `degree_centrality` : 次数中心性
- `betweenness_centrality` : 媒介中心性
- `community_id` : コミュニティ検出結果（metadata に community ラベルを含む）

### node_aliases（オントロジー統合との共有）

| Column | Type | Description |
| --- | --- | --- |
| canonical_node_id | STRING | 正規ノード識別子 |
| alias_node_id | STRING | 統合元ノード識別子 |
| alias_label | STRING | 表記揺れラベル |
| similarity_score | FLOAT64 | 類似スコア |
| merge_status | STRING | 統合状態 |
| created_at | TIMESTAMP | 作成日時 |

#### merge_status Values

- `suggested` : 自動候補（未レビュー）
- `approved` : 承認済み（グラフクエリ時に canonical に読み替える）
- `rejected` : 却下

## グラフエンジンの選択

### 正本・分析: BigQuery + Cloud Run Jobs

- `BigQuery` に canonical 化前後の node と評価結果を保存する
- NetworkX / igraph を Cloud Run Jobs で動かし、計算結果を BQ に書き戻す
- `/dev/stats`、評価トレンド、再処理、監査は `BigQuery` を参照する

BigQuery 側では node metadata と alias 情報を集計対象とし、経路探索そのものは初期実装では `Spanner Graph` に委譲する。

### 探索: Spanner Graph

- GQL（Graph Query Language）ネイティブ対応で多段ノードトラバーサルが直感的に書ける
- API には edge を露出しないが、内部では canonical node 間の exploration adjacency を持つ
- adjacency は tree 上の親子、summary 内リンク、canonical cluster 近傍から生成する
- `BigQuery` との二重持ちになるが、探索系クエリ特性が大きく異なるため併用する
- `BigQuery` を正本・分析用途、`Spanner Graph` をリアルタイム探索用途で使い分ける

```
-- Spanner Graph GQL の例
GRAPH ActionRevGraph
MATCH (start {id: @start_node_id})-[*1..5]->(m)
RETURN m.label, m.type
```

### Node-only Exploration Model

- `FindPaths` は `node_ids` の列だけを返し、hop は隣接 canonical node の遷移回数として扱う
- adjacency 自体は `Spanner Graph` の内部表現であり、公開 proto には含めない
- 1 adjacency は「ユーザーが 1 回の探索操作で自然に移動できる node 間近接」を意味する
- source documents は path 全体の根拠候補として `PathEvidenceRef.source_document_ids` に集約する

### 同期方針

- canonical 化が確定した node と exploration adjacency を `BigQuery` 由来の情報から `Spanner Graph` に同期する
- 探索用 graph には `approved` alias のみ反映する
- 同期は document 完了後と再処理後に実行する
- 一時的な同期遅延は許容するが、評価・監査・運用判断は常に `BigQuery` を正とする

## フロントエンドへの反映

- `GetGraph` のレスポンスに `node_scores` を JOIN して返す
- 初期は `pagerank` スコアを paper-in-paper の `importanceMap` に反映し、重要度によるレイアウト優先度に使う
- `community_id` による色分けはメタデータパネルで表示し、初期リリースの主可視化には含めない
- 中心性の詳細値はメタデータパネルで表示し、複数スコアの同時可視化は初期スコープ外とする
- `FindPaths` の結果は path の node_ids を順にペーパーとして展開（OPEN_NODE）し、hop 数をパスバーに表示する
- フロントは adjacency の種別を知らず、path 上の `node_ids` を順に展開するだけでよい

## 実行タイミング

| タイミング | 用途 |
| --- | --- |
| document 処理完了後に即時実行 | 差分更新（追加 document に関連する PageRank / community_id の再計算と Spanner Graph 同期） |
| 夜間バッチ（BigQuery scheduled query）| 横断グラフ全体の再計算 |
| 手動トリガー | 再処理・デバッグ |

### 初期スコープのアルゴリズム

- document 完了時の差分更新では `pagerank` と `community_id` のみを計算する
- `degree_centrality`, `betweenness_centrality`, 最短経路系は夜間バッチまたは将来対応とする

### トリガー方針

- document 処理完了後は Pub/Sub イベントを起点に Cloud Run Jobs を起動する
- 夜間全体再計算は BigQuery scheduled query または Cloud Scheduler から Cloud Run Jobs を起動する
- 手動トリガーは運用者向け管理コマンドまたは `/dev/stats` から実行する
- Spanner Graph 同期ジョブも同じイベントチェーンで起動する

### 再計算スコープ

- 即時実行では、新規 document に紐づく canonical node とその 2-hop 近傍を差分再計算対象とする
- 夜間バッチでは全 canonical graph を再計算し、差分更新で発生したドリフトを解消する
- 差分更新に失敗した場合は document 処理自体を失敗にせず、夜間バッチでの回復に委ねる

### Spanner Graph の扱い

- 初期から `BigQuery` と `Spanner Graph` を併用する
- `GetGraph` は `BigQuery`、`FindPaths` は `Spanner Graph` を主参照先とする
- 集計値やランキングの正本は `BigQuery` に保持し、探索時に必要な最小限の属性だけを `Spanner Graph` に複製する

## 処理フローの全体像

```
[正規化 → 抽出]
      ↓
[ノード統合（document 内）]
      ↓
[トピックマッピング（heuristic → LLM）]
      ↓
[トピック canonical 化（node_aliases）]
      ↓
[横断グラフ構築]
      ↓
[BigQuery 正本保存]
      ↓
[Spanner Graph 同期]
      ↓
[グラフアルゴリズム適用（Cloud Run Jobs）]
      ↓
[可視化（React + @keyhole-koro/paper-in-paper）]
```

## Open Issues

- 差分更新の 2-hop 近傍で十分か、より広い伝播範囲が必要か
