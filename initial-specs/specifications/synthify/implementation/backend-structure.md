# Backend Structure: Synthify バックエンド実装設計

## 1. 概要
Synthify のバックエンドは Go 言語で実装され、[Connect RPC](https://connectrpc.com/) を通信プロトコルとして使用する。データストアには Google Cloud Spanner を使用し、大規模な Paper Tree データの保存と検索を効率化する。

## 2. ディレクトリ構成 (src/backend)
クリーンアーキテクチャに基づいたレイヤー分割を行う。

```text
/home/unix/Synthify/src/backend/
├── cmd/
│   └── server/                # エントリポイント
├── internal/
│   ├── domain/                # エンティティ、リポジトリ、インターフェース定義
│   │   ├── paper.go           # Paper 構造体
│   │   └── repository.go      # Spanner 操作インターフェース
│   ├── usecase/               # ビジネスロジック（ユースケース）
│   │   ├── extraction_service.go  # Gemini 3 連携による Paper Tree 生成
│   │   └── tree_view_service.go   # フロントエンドへのツリーデータ提供
│   ├── infrastructure/        # 外部ツール、フレームワーク実装
│   │   ├── spanner/           # Spanner クライアント実装
│   │   └── gemini/            # Gemini 3 API クライアント
│   └── transport/
│       └── connect/           # Connect RPC ハンドラ実装
├── proto/                     # Protocol Buffers 定義
│   └── synthify/graph/v1/    # API 定義
└── Makefile                   # ビルド、生成コマンド
```

## 3.主要コンポーネントの役割

### 3.1. Extraction Service (抽出サービス)
- **役割**: Gemini 3 API を呼び出し、ドキュメントから Paper Tree JSON を取得する。
- **処理フロー**:
  1. ソースドキュメントを読み込み、プロンプトを構築。
  2. Gemini 3 にリクエストし、JSON をパース。
  3. 各 Paper に UUID を付与（必要に応じて）。
  4. Spanner に一括保存。

### 3.2. Tree View Service (ツリービューサービス)
- **役割**: フロントエンド（Paper-in-Paper）が必要な形式でデータを取得・配信する。
- **機能**:
  - `GetRootPapers`: ルート階層の Paper リストを取得。
  - `GetPaperRoom`: 特定の `parent_id` を持つ子 Paper のリストを取得（部屋単位の遅延読み込み）。
  - `SearchPapers`: 全文検索による Paper 探索。

### 3.3. Spanner Repository (データ永続化)
- **役割**: Cloud Spanner へのクエリ実行。
- **スキーマ管理**: 
  - `papers` テーブル: `id`, `parent_id`, `child_ids`, `title`, `description`, `content_html`, `importance` 等。

## 4. API 定義 (Protobuf)
`contract/api-spec.md` に基づき、Connect RPC 用の定義を作成する。

```proto
service PaperService {
  rpc GetPaperTree(GetPaperTreeRequest) returns (GetPaperTreeResponse);
  rpc GetPaperRoom(GetPaperRoomRequest) returns (GetPaperRoomResponse);
  rpc CreatePaperTree(CreatePaperTreeRequest) returns (CreatePaperTreeResponse);
}
```

## 5. 今後の課題
- **トランザクション管理**: 一度の抽出で大量の Paper が生成されるため、Spanner のミューテーション制限（Mutation Limit）を考慮したバッチ挿入の実装。
- **キャッシュ戦略**: 頻繁にアクセスされる Paper データのキャッシュ（Redis または In-memory）の導入検討。
- **認証・認可**: 各 Paper へのアクセス権限管理（IAM またはカスタム ACL）。
