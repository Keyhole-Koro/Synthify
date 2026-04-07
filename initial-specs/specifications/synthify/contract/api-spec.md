# 05. API Specification (Synthify)

## Overview

Synthify の API は `Connect RPC` (gRPC 互換) を使用し、フロントエンドとバックエンド間の通信を行う。

## Core RPCs

### GetPaperTree

ドキュメント全体の Paper Tree 構造（メタデータのみ）を取得する。

- **Request**: `{ document_id: string }`
- **Response**:
  - `document_id`: ドキュメント識別子
  - `papers`: `PaperMetadata` の配列
    - `id`: Paper ID
    - `parent_id`: 親 Paper ID
    - `title`: タイトル
    - `child_ids`: 子 Paper ID の配列

### GetPaper

特定の Paper の詳細（HTML コンテンツを含む）を取得する。

- **Request**: `{ document_id: string, paper_id: string }`
- **Response**:
  - `id`: Paper ID
  - `title`: タイトル
  - `description`: 概要テキスト
  - `content_html`: iframe 内で表示する HTML（`<a data-paper-id="...">` リンクを含む）
  - `child_ids`: 子 Paper ID の配列

### UpdatePaperLayout

ユーザーが手動で行ったレイアウトの変更を保存する。

- **Request**:
  - `document_id`: ドキュメント識別子
  - `parent_paper_id`: 部屋（親）の ID
  - `layouts`: `PaperLayout` の配列
    - `paper_id`: 子 Paper の ID
    - `grid_x`, `grid_y`, `grid_w`, `grid_h`: 位置とサイズ
    - `is_opened`: 展開状態

## Protocol Buffers Definition (Candidate)

```proto
syntax = "proto3";

package synthify.synthify.v1;

// Paper のメタデータ（ツリー構造の構築に必要最小限のデータ）
message PaperMetadata {
  string id = 1;
  optional string parent_id = 2; // ルートの場合は空
  string title = 3;
  repeated string child_ids = 4; // この部屋に所属する子の ID リスト
}

// Paper の詳細データ（コンテンツを含む）
message PaperDetail {
  string id = 1;
  string title = 2;
  string description = 3;
  string content_html = 4; // iframe 内で表示する HTML
  repeated string child_ids = 5;
}

// 部屋（親）の中での子のレイアウト情報
message PaperLayout {
  string paper_id = 1;
  int32 grid_x = 2;
  int32 grid_y = 3;
  int32 grid_w = 4;
  int32 grid_h = 5;
  bool is_opened = 6;
}

message GetPaperTreeRequest {
  string document_id = 1;
}

message GetPaperTreeResponse {
  string document_id = 1;
  repeated PaperMetadata papers = 2; // フラットな配列だが parent_id/child_ids で木を構成
}

message GetPaperRequest {
  string document_id = 1;
  string paper_id = 2;
}

message UpdatePaperLayoutRequest {
  string document_id = 1;
  string parent_paper_id = 2; // 更新対象の「部屋」の ID
  repeated PaperLayout layouts = 3;
}

message UpdatePaperLayoutResponse {}

service PaperService {
  // ドキュメント全体の構造（メタデータ）を取得
  rpc GetPaperTree(GetPaperTreeRequest) returns (GetPaperTreeResponse);

  // 特定のペーパーの詳細（コンテンツ）を取得
  rpc GetPaper(GetPaperRequest) returns (PaperDetail);

  // 部屋内での配置・状態変更を保存
  rpc UpdatePaperLayout(UpdatePaperLayoutRequest) returns (UpdatePaperLayoutResponse);
}
```

## Error Handling

- `NOT_FOUND`: 指定された document または paper が存在しない。
- `PERMISSION_DENIED`: ユーザーが対象の workspace にアクセス権を持っていない。
- `FAILED_PRECONDITION`: 文書の解析が完了していない。
