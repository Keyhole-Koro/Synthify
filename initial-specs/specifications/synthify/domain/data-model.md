# 04. Data Model (Synthify)

## Tables

### users / workspaces / documents

これらのテーブルは、基本的な管理・認証機能として `gcp-graph-system` と共通の構造を維持する。

### papers

従来の `nodes` と `edges`（階層関係）を統合・拡張した、システムの中心的なテーブル。

| Column | Type | Description |
| --- | --- | --- |
| document_id | STRING | ドキュメント識別子 |
| paper_id | STRING | Paper 識別子（`pa_<ULID>` 形式） |
| parent_id | STRING | 親 Paper ID。Root の場合は null |
| title | STRING | 表示タイトル |
| description | STRING | 概要テキスト（ホバー時にポップアップ表示） |
| content_html | STRING | メインコンテンツ（iframe 用の HTML） |
| child_ids | ARRAY<STRING> | 子 Paper ID の配列。順序を保持する |
| importance | FLOAT64 | 初期重要度スコア。時間経過で減衰し、操作で増加する |
| source_chunk_id | STRING | 出典 chunk ID |
| created_at | TIMESTAMP | 作成日時 |
| updated_at | TIMESTAMP | 更新日時 |

- `content_html` 内の `<a data-paper-id="...">` は `child_ids` に含まれる ID を参照しなければならない。
- `child_ids` には存在するが `content_html` にリンクがない Paper は、部屋の中に「閉じたカード」として配置される。

### paper_layouts

各部屋（Parent Paper が作る空間）における子 Paper の空間配置情報を保持する。

| Column | Type | Description |
| --- | --- | --- |
| parent_paper_id | STRING | 部屋（親）の ID |
| paper_id | STRING | 配置される子 Paper の ID |
| grid_x | INT64 | Grid 上の X 座標 |
| grid_y | INT64 | Grid 上の Y 座標 |
| grid_w | INT64 | Grid 上の幅（重要度から派生するが表示サイズとして保存） |
| grid_h | INT64 | Grid 上の高さ |
| is_opened | BOOLEAN | 現在展開されているかどうか |
| last_accessed_at | TIMESTAMP | 最終アクセス（フォーカス・展開）日時 |

- ユーザーが手動でドラッグした場合にこの情報が更新される。
- 手動配置がない場合は、重要度に基づく自動配置が行われる。

### document_chunks

`gcp-graph-system` と同様に、テキストの出典管理に利用する。

## State Families (Synthify Specific)

| Family | 用途 | 主な値 |
| --- | --- | --- |
| `PaperExpansionState` | Paper の展開状態 | `closed` (カード表示) / `opened` (iframe表示) |
| `ImportanceDecayRule` | 重要度の減衰ルール | 二乗則に基づく減衰率の定義 |

## Naming Rules

- `Paper` は `gcp-graph-system` における `Node` に相当するが、単なる点ではなく「コンテンツ（HTML）を持つ面」であることを強調する。
- `Room` は特定の Parent Paper の `child_ids` が作る空間的なコンテキストを指す。
