# 01. Overview (Synthify)

## Purpose

本仕様書は、ファイルから Gemini を用いて意味構造（Paper Tree）を抽出し、階層的な「部屋（Room）」と「紙（Paper）」の形式で可視化・探索する Web システム **Synthify** の要件、構成、データ設計、API、運用方針を定義する。

`paper-in-paper` ライブラリを採用し、文書内の概念を再帰的にネストされた空間的な構造として表現することで、読者が能動的に情報の粒度を制御できる読書体験を提供する。

## Goals and Non-Goals

### Goals

- ユーザーがファイルをアップロードできること
- ファイルからテキストを抽出し、Gemini で再帰的な Paper 構造（Parent-Child）を生成できること
- 各 Paper が HTML コンテンツ（iframe 内レンダリング）を持ち、関連する子 Paper へのリンクを含むこと
- `paper-in-paper` ライブラリを用いて、部屋と紙のメタファーで情報を提示すること
- 重要度（Importance）に基づいた自動的なレイアウトと、情報の自動縮小（Auto-shrinking）を実現すること
- 出典となる chunk を追跡できること
- 初期実装は低運用負荷で立ち上げられること

### Non-Goals

- 初期段階でのリアルタイム共同編集
- 初期段階での完全自動オントロジー統合
- グラフ全体を一度に俯瞰するマップ表示（代わりに再帰的な部屋遷移を用いる）

## Users and Use Cases

### Primary Users

- 分析担当者
- 企画担当者
- 文書の構造化・深掘りを行いたい業務ユーザー

### Primary Use Cases

1. ユーザーがファイルをアップロードする
2. システムがファイルを解析して再帰的な Paper 構造（Tree）を生成する
3. ユーザーが Root Paper から順に「部屋」を探索する
4. ユーザーが文中のリンクをクリックして、子 Paper をその場で展開する
5. ユーザーがドラッグで部屋内の Paper の配置を変更し、自分なりの整理を行う
6. システムが重要度の低い（閲覧されていない）Paper を自動的に縮小し、スペースを管理する

## Release Scope Summary

### Initial Release

- 単一ファイルアップロード
- PDF、Markdown、TXT、CSV 対応
- Gemini による再帰的 Paper 抽出
- `BigQuery` 保存
- `Spanner Graph` への探索用同期（ツリー構造として保持）
- `paper-in-paper` によるフロントエンド表示
- リンククリックによる子 Paper 展開
- 重要度ベースの自動縮小
- ノード詳細と出典表示
- フロントは `TypeScript + React + paper-in-paper`
- バックエンドは `Go + Connect RPC`

### Deferred Items

- 大量ファイル同時処理
- 横断検索
- 高度なアクセス制御
- 部屋を跨いだ Paper の複雑なリンク関係（純粋なツリー構造を優先）
