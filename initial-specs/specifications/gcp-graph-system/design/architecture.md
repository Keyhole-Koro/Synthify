# 03. Architecture

## System Overview

本システムは、ユーザーがアップロードしたファイルを `Cloud Storage` に保存し、`Cloud Run` 上の `Go` バックエンドが処理を制御する。AI パイプラインでは、まず正規化ツール層が原本を整形し、その後テキスト抽出と chunk 分割を行う。必要に応じて `Vertex AI Gemini` に整形スクリプト生成を依頼し、生成された Python ツールをサンドボックスで dry-run または実行する。その後 `Vertex AI Gemini` により意味構造を JSON 形式で抽出し、`PostgreSQL` に document / chunk / node / edge / evidence を正本として保存する。フロントエンドが必要とする `paper-in-paper` の表示は document 単位の tree と補助的な横断 edge で成立するため、初期は graph database を使わず `PostgreSQL` 上の取得とアプリケーション側の探索ロジックで対応する。フロントエンドは `TypeScript + React + @keyhole-koro/paper-in-paper` で実装し、`Firebase Hosting` から配信する。フロントエンドとバックエンド間の同期通信は `Connect RPC` を用いる。

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
           +--> [PostgreSQL]
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
- `PostgreSQL`: document / chunk / node / edge / workspace / membership / view 履歴の正本保存
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
12. `PostgreSQL` に document/chunk/node/edge を保存する
13. フロントエンドが `Connect RPC` でグラフ取得・経路検索を行い、`paper-in-paper` でペーパーツリーとして表示する

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
- PostgreSQL 書き込み
- グラフ取得
- 近傍探索 / 経路探索（初期はアプリケーション側ロジックまたは recursive query）
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

### Primary Relational Store

#### Responsibility

- workspace/document/chunk/node/edge/view の永続化
- API の一次参照元
- 再処理、監査、管理操作の基盤

#### Service

- `PostgreSQL`

#### Notes

- `paper-in-paper` の主要要件は document 単位の tree 表示と補助的な横断 edge で満たせるため、初期段階では graph database を導入しない
- `FindPaths` や cross-document exploration が性能上のボトルネックになった場合のみ、探索専用ストアの追加を再検討する
