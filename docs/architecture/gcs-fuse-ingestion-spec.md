# GCS FUSE Storage & Ingestion Specification

## 状態: 確定 (Confirmed)
最終更新日: 2026-05-07

## 概要
本ドキュメントは、LLM Worker (Cloud Run) における Google Cloud Storage (GCS) のマウント、およびドキュメントのインジェクション（取り込み）フローに関する技術仕様を定義する。
Cloud Storage FUSE を活用することで、大規模ドキュメントの効率的な処理、低コストな検索、および堅牢な出所追跡（Provenance）を実現する。

## 1. ディレクトリ構造仕様

全てのドキュメントは、マウントパスをルートとする「プロジェクトディレクトリ」として一貫した構造で保持される。

### マウントパス
本番環境: `/mnt/gcs/`
開発環境: 環境変数 `GCS_FUSE_MOUNT_PATH` で指定されたローカルパス

### ドキュメントの実体構造

`{workspace_id}/{document_id}/` は**常にディレクトリ**であり、役割の異なる2つの
サブツリーを持つ。原本（アップロード）と作業領域（worker 展開）を分離することで、
ドキュメントルートが同名ファイルと衝突しない（旧構造ではこの衝突で extract が必ず
失敗していた。[gcs-storage-layout-redesign](../improvements/gcs-storage-layout-redesign.md)）。

```
{mount_path}/{workspace_id}/{document_id}/
├── source/                  ← 原本。API がアップロード、worker は読むだけ
│   └── original.{ext}          単一ファイル（ZIP は archive.zip）
└── extracted/               ← 作業領域。worker が展開・書き込み
    └── {relative_path}         単一: 原本のコピー / ZIP: 展開した階層
```

- **アップロード先**: `{workspace_id}/{document_id}/source/original.{ext}`。
  ファイル名はパスに使わず（パス事故防止）、元名は `documents.filename` に保持。
- **単一ファイル**: worker が `source/` の原本を `extracted/` にコピー。
- **アーカイブ（ZIP等）**: `source/archive.zip` を `extracted/` 配下に階層を維持して展開。
- **検索（grep_search）**: `extracted/` を対象に再帰検索する。

### キャッシュおよびメタデータ構造
`{mount_path}/.cache/v1/{document_id}/{category}/{key}.json`
`{mount_path}/.checkpoints/{job_id}/{stage}.json`

## 2. データベース正規化仕様

ドキュメント内の各ファイルは `document_files` テーブルで厳格に管理される。

### 構成
- `documents`: ドキュメント全体のコンテナ（zipファイル等に対応）。
- `document_files`: ディレクトリ内の個別の物理ファイル。
- `document_chunks`: 各ファイルから抽出されたテキスト断片。必ず `file_id` (NOT NULL) で所属ファイルを指し示す。

## 3. インジェクション・フロー (extract_text)

`extract_text` ツールは以下の手順でドキュメントを正規化する。原本 `source/` は
読むだけで、書き込みは作業領域 `extracted/` に対して行う。

1.  **読み出し**: `{wsID}/{docID}/source/` 配下の原本を読む。
2.  **確保**: `{wsID}/{docID}/extracted/` ディレクトリを作成。
3.  **展開/配置**:
    - **ZIPの場合**: `extracted/` 配下に再帰展開し、各ファイルに `document_files` レコードを作成。
    - **単一ファイルの場合**: `extracted/` に保存し、単一の `document_files` レコードを作成。
4.  **判定 (Heuristics)**:
    - 先頭 512 バイトの NULL バイトスキャンおよび UTF-8 妥当性チェックにより、拡張子に寄らずテキスト/バイナリを判別。
5.  **出力**: 各ファイルパスと ID を含んだ構造化テキスト（Marker付き）を LLM に返す。

## 4. 検索および分析仕様

### キーワード検索 (grep_search)
- **実装**: `extracted/` ディレクトリに対してシステムコマンド `grep -rn -H` を実行。
- **ID解決**: ヒットしたパスを `document_files` テーブルで解決し、`file_id` を付加して返す。
- **キャッシュ**: 検索結果を `.cache/` に保存し、再検索をミリ秒単位で完了させる。

### ベクトル検索 (semantic_search)
- **実装**: DB (pgvector) による近似最近傍探索。
- **結果**: ヒットしたチャンクに紐づく `file_id` を介して、JOIN によりファイルパスを取得。

## 5. 運用上の制約と考慮事項

- **Last-writer-wins**: GCS FUSE の `rename` 特性に基づき、キャッシュ競合時は上書きを許容する。
- **アトミック性**: 書き込み時は常に `.tmp` ファイルを作成し、完了後に `rename` を行う。
- **ReadOnly マウント**: 原本 `{workspace_id}/{document_id}/source/` は worker からは
  読むだけなので ReadOnly 化が望ましい。一方 worker は `extracted/` `.cache/`
  `.checkpoints/` に書くため、マウント全体を ReadOnly にはできない（per-path RO は
  gcsfuse 上の追加検討事項。現状はマウント RW）。

## 関連ドキュメント
- [llm-worker-architecture.md](llm-worker-architecture.md)
- [job-entity-field-spec-status.md](../improvements/job-entity-field-spec.md)
