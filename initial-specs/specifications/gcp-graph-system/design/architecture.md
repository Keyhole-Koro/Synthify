# 03. Architecture

## System Overview

本システムは、ユーザーがアップロードしたファイルを `Cloud Storage` に保存し、`Cloud Run` 上の `Go` バックエンドが処理を制御する。AI パイプラインでは、まず正規化ツール層が原本を整形し、その後テキスト抽出と chunk 分割を行う。必要に応じて `Vertex AI Gemini` に整形スクリプト生成を依頼し、生成された Python ツールをサンドボックスで dry-run または実行する。その後 `Vertex AI Gemini` により意味構造を JSON 形式で抽出し、`BigQuery` にノード・メタデータを正本として保存する。canonical 化済みの探索用ノード集合は `Spanner Graph` に同期し、多段経路検索に利用する。ここで `Spanner Graph` は API に露出する edge を保持するためではなく、canonical node 間の exploration adjacency を高速に辿るための内部探索ストアとして使う。フロントエンドは `TypeScript + React + @keyhole-koro/paper-in-paper` で実装し、`Firebase Hosting` から配信する。フロントエンドとバックエンド間の同期通信は `Connect RPC` を用いる。

## High-Level Architecture

```text
[Browser]
   |
   v
[Firebase Hosting]
   |
   +--> React + paper-in-paper UI
           |
           v
   [Connect RPC Client]
           |
           v
   [Cloud Run Go Backend]
           |
           +--> [Cloud Storage]
           +--> [Tool Registry]
           +--> [Sandbox Runner]
           +--> [Vertex AI Gemini]
           +--> [BigQuery]
           +--> [Spanner Graph]
           |
           +--> [Cloud Tasks] optional
```

## Minimal GCP Components

- `Firebase Hosting`: `TypeScript + React + @keyhole-koro/paper-in-paper` フロントエンド配信
- `Cloud Run`: `Go + Connect RPC` バックエンド、テキスト抽出、ノード化処理
- `Cloud Storage`: 元ファイル保存
- `Tool Registry`: 正規化ツール定義と履歴保存
- `Sandbox Runner`: Python 正規化ツールの隔離実行
- `Vertex AI Gemini`: ノード抽出と HTML サマリ生成
- `BigQuery`: ドキュメント、chunk、ノード、評価、統計の正本保存
- `Spanner Graph`: 探索用 adjacency store、多段経路検索
- `Protocol Buffers`: RPC スキーマ定義

## Logical Processing Flow

1. ユーザーがファイルをアップロードする
2. API が `Cloud Storage` にファイルを保存する
3. `documents` にメタデータを作成する
4. 必要に応じて LLM がデータ整形用 Python ツール案を生成する
5. 生成または既存の正規化ツールをサンドボックスで dry-run または実行する
6. 正規化済みドキュメントを保存する
7. API がテキスト抽出を実行する
8. テキストを chunk に分割する
9. chunk を Gemini に送信する
10. Gemini が JSON 形式でノード候補を返す
11. API が重複統合と正規化を実施する
12. `BigQuery` に document/chunk/node を保存する
13. canonical 化済み node と exploration adjacency を `Spanner Graph` に同期する
14. フロントエンドが `Connect RPC` でグラフ取得・経路検索を行い、`paper-in-paper` でペーパーツリーとして表示する

## Component Specification

### Frontend

#### Responsibility

- ファイルアップロード UI
- 解析ステータス表示
- ペーパーツリー形式でのノード表示（`@keyhole-koro/paper-in-paper`）
- ノード詳細・出典表示
- ペーパー内コンテンツリンクによる関連ペーパー展開
- 経路検索と展開操作への変換

#### Stack

- `TypeScript`
- `React`
- `@keyhole-koro/paper-in-paper`

#### Hosting

- `Firebase Hosting`

#### Future Enhancements

- 認証連携
- フィルタ UI
- 検索 UI

### API and Processing Layer

#### Responsibility

- `Connect RPC` エンドポイントの提供
- ファイル受領
- 正規化ツール生成
- 正規化ツール実行管理
- テキスト抽出
- chunk 分割
- Gemini 呼び出し
- ノード統合
- BigQuery 書き込み
- Spanner Graph 同期
- グラフ取得
- 近傍探索 / 経路探索
- 非同期ジョブ投入

#### Stack

- `Go`
- `Connect RPC`
- `Protocol Buffers`

#### Runtime

- `Cloud Run`

#### Initial Design Decision

- 同期の問い合わせ系は `Connect RPC` で提供する
- 重いノード化処理はジョブ起動型とし、初期は単純実装、将来的に `Cloud Tasks` へ移行する

### File Storage

#### Responsibility

- 元ファイルの永続保存
- 再処理時の原本参照

#### Service

- `Cloud Storage`

### LLM Processing

#### Responsibility

- データ整形用 Python スクリプト案の生成
- テキストから階層付きノードと関係を抽出する
- 出典 chunk を付与した JSON を返す

#### Service

- `Vertex AI Gemini`

#### Constraints

- JSON Schema に沿った出力を要求する
- モデル出力のゆらぎを前提にアプリ側で正規化する
- 生成スクリプトは直接本番適用せず、サンドボックスで検証できること

### Tool Registry and Sandbox

#### Responsibility

- 正規化ツールの保存
- ツール metadata、version、approval 状態の管理
- ツールの dry-run と本実行
- 実行結果、差分、失敗ログの記録

#### Runtime

- `Cloud Run` または `Cloud Run Jobs`
- Python 実行環境を隔離したコンテナ

#### Constraints

- ネットワークを原則無効化する
- 入力ディレクトリ以外の読み取りを制限する
- 出力先を作業ディレクトリに限定する
- 実行時間、メモリ、subprocess 利用を制限する

### Structured Data Store

#### Responsibility

- document/chunk/node の永続化
- 分析、再集計、再処理の基盤

#### Service

- `BigQuery`

### Graph Query Store

#### Responsibility

- canonical 化済み graph の保持
- 近傍展開、到達可能性、多段経路検索
- 対話的な graph traversal の低レイテンシ提供

#### Service

- `Spanner Graph`

#### Constraints

- 正本は `BigQuery` とし、`Spanner Graph` は探索向けの複製とする
- `Spanner Graph` には canonical node と exploration adjacency だけを持たせ、公開 API の契約は node-only に保つ
- 同期遅延があっても、評価・再処理・監査は常に `BigQuery` を参照する
