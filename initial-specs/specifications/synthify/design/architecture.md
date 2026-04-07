# 03. Architecture (Synthify)

## System Overview

Synthify は、GCP のサーバレス環境をベースとしたドキュメント解析・空間的可視化システムである。

- **Backend**: Go + Connect RPC (Cloud Run)
- **Database**: BigQuery (正本保存) / Spanner Graph (探索・ツリー走査用)
- **Frontend**: Next.js + React + `paper-in-paper` ライブラリ
- **AI**: Gemini 1.5 Pro / Flash (構造化抽出と HTML 生成)

## Analysis Pipeline (Paper Tree Extraction)

ドキュメント（PDF, Markdown 等）から Paper Tree を抽出するプロセス。

1. **Intake**: ユーザーがファイルを GCS にアップロード。
2. **Text Extraction**: Cloud Run 上でテキストとレイアウト情報を抽出。
3. **Semantic Chunking**: 意味的なまとまりごとに Chunk 分割。
4. **Recursive Structure Generation (Gemini)**:
   - Chunk 群を Gemini に入力。
   - 文書全体の「階層構造」を定義（Root, Chapters, Sections, Details）。
   - 各ノードを `Paper` として定義し、以下の要素を生成:
     - **Title**: 短い名前。
     - **Description**: マウスホバー用の概要。
     - **Content HTML**: 子 Paper への `data-paper-id` リンクを含む構造化 HTML。
     - **Child IDs**: その部屋に所属する子 Paper の ID リスト。
5. **Persistence**: 抽出結果を BigQuery および Spanner に保存。

## Frontend Integration (paper-in-paper)

`paper-in-paper` ライブラリを活用した UI アーキテクチャ。

- **Paper Store Context**: 全ての Paper の状態（展開状態、重要度、レイアウト）を管理。
- **Room Engine**:
  - `parent_id` に紐付く子 Paper たちを Grid 上に配置。
  - 重要度（Importance）に基づき、各 Paper のサイズを動的に計算。
  - スペース不足時に自動縮小（Shrink）を発火。
- **Iframe Bridge**:
  - 各 Paper は独立した iframe でレンダリング。
  - iframe 内の `data-paper-id` リンククリックを `postMessage` で親ウィンドウへ通知。
  - 親ウィンドウ（Paper Store）が該当する子 Paper を「展開（Open）」状態に変更。

## AI Prompting Strategy

Gemini に「Paper Tree」を生成させるためのプロンプト戦略。

- **Prompt 1 (Structural Mapping)**: 文書全体のトピックマップ（Tree 構造）を先に抽出。
- **Prompt 2 (Content Generation)**: 各トピック（Paper）に対して、本文を HTML 形式で生成。その際、定義済みの `Child IDs` へのリンクを適切に埋め込む。

## Scalability and Optimization

- **Lazy Loading**: ユーザーが Root 部屋から順に掘り下げるため、下位階層の `content_html` は必要になるまで取得しない（遅延読み込み）。
- **State Management**: 全ての Paper の展開状態を React Context または Jotai/Zustand で管理し、効率的な再描画を実現。
