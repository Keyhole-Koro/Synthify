# GCS ストレージ・レイアウト再設計（原本/作業領域の分離）

> **Status: 実装済み（2026-06-02）。** `{ws}/{doc}/source/` と
> `{ws}/{doc}/extracted/` に分離。API のアップロード先・worker の読み出し/展開/grep・
> Fake GCS 経路・spec をすべて新構造に更新。後方互換なし。stage の既存 job は
> リセット前提。残作業: 下記「残課題」と、既存データの実リセット。

## 背景：現状は構造的に破綻している

2026-06-01、stage で新規 job がことごとく抽出に失敗していた。ログ:

```
orchestrator.document_map_extract_failed:
  failed to create document dir: mkdir /mnt/gcs/{ws}/{doc}: not a directory
```

GCS 上の実態:
```
gs://bucket/{workspace_id}/{document_id}    ← これがアップロードされた「ファイル」
```

### 何が衝突しているか

| 層 | コード | パス前提 |
|---|---|---|
| API アップロード | [document.go](../../apps/api/internal/repository/postgres/document.go) → `IssueDocumentUploadURL(ctx, ws, docID, …)` → object `{ws}/{doc}` | **`{ws}/{doc}` はファイル** |
| 仕様 | [gcs-fuse-ingestion-spec.md](../architecture/gcs-fuse-ingestion-spec.md) | `{ws}/{doc}/{relpath}`（`{doc}` はディレクトリ） |
| worker 読み出し | `ReadAll` = `DocPath(ws,doc)` をファイルとして読む | **ファイル**（API に合致） |
| worker 抽出 | `runExtraction` / `processZip` = `MkdirAll(DocPath(ws,doc))` | **ディレクトリ**（仕様に合致） |

worker 自身が二枚舌：同じ `{ws}/{doc}` を「読むときはファイル」「展開するときはディレクトリ」
として扱う。`{ws}/{doc}` がファイルなので `MkdirAll` が必ず `not a directory` で失敗し、
**extract_text は単一ファイルでも ZIP でも構造的に必ず詰まる**。

### ZIP でさらに明白

[processZip](../../apps/worker/pkg/worker/tools/builtin/io/extraction.go) は ZIP の
バイト列を `{ws}/{doc}`（ファイル）から読み、同じ `{ws}/{doc}/` をディレクトリとして
作って中に展開しようとする。**原本 ZIP の置き場所と展開先ディレクトリが同一パスを
奪い合う。** 設計の時点で ZIP は処理不能だった。

### 仕様自体の自己矛盾

[spec](../architecture/gcs-fuse-ingestion-spec.md) は「worker が `{doc}/` ディレクトリを
作成し原本を配置」(L41-44) と書く一方、「原本（`{ws}/` 直下）は ReadOnly マウント
したい」(L64) とも書く。ReadOnly なら worker は mkdir も書き込みもできない。両立しない。

## 再設計：原本と作業領域を別パスに分離する

矛盾の根は「**アップロード原本**」と「**worker の展開作業領域**」を同じ `{ws}/{doc}` に
重ねていること。これを分ける。`{ws}/{doc}/` は**常にディレクトリ**とし、その下に役割の
違う2つのサブツリーを持つ。

```
{mount}/{workspace_id}/{document_id}/
├── source/                  ← 原本。API がアップロード、worker は読むだけ（ReadOnly 可）
│   ├── {filename}              単一ファイル: アップロードされたファイル名のまま
│   └── archive.zip            ZIP: 原本をそのまま置く
└── extracted/               ← 作業領域。worker が書く
    └── {relpath}              単一: source のコピー / ZIP: 展開した階層
```

- `{ws}/{doc}/` がファイルになることは二度と起きない（常にディレクトリ）
- 原本 `source/` と作業 `extracted/` が別なので、原本 ReadOnly マウントが成立する
- 単一ファイルと ZIP が**同じ構造**で扱える（単一は source に1個、extracted に1個）

### document_files との整合

[document_files](../architecture/gcs-fuse-ingestion-spec.md) は「1ドキュメント=複数物理
ファイル」を表すテーブル。`extracted/{relpath}` の各ファイルが1レコードに対応する。
単一ファイルなら1レコード、ZIP なら展開後のファイル数だけレコード。現行の
`CreateDocumentFile(docID, relPath, …)` の `relPath` を `extracted/` 起点に統一する。

## 変更が必要な箇所

### API（アップロード先を source/ に）

- `IssueDocumentUploadURL` の objectName を `docID` から
  `{docID}/source/{filename}` に変更
  （[document.go:153](../../apps/api/internal/repository/postgres/document.go)）。
  filename をアップロード要求から受け取る必要がある（現状 docID だけ渡している点の見直し）
- メタデータ取得 `GetObjectMetadata` / 削除 `DeleteDocumentObject` の object 名も
  追従（[metadata.go](../../apps/api/internal/infrastructure/storage/metadata.go)）。
  削除は `{ws}/{doc}/` プレフィックス配下の一括削除に

### worker storage（パス計算の分離）

- `DocPath(ws, doc)` を**ディレクトリ**の意味に固定し、用途別に
  `SourceDir` / `ExtractedDir` を追加（[filesystem.go](../../apps/worker/pkg/worker/storage/filesystem.go)）
- `ReadAll` / `PopulateSourceFile` を「`source/` 配下の原本を読む」に変更。
  単一ファイルは source/ の唯一のファイル、ZIP は source/archive.zip

### worker extraction（mkdir/展開先を extracted/ に）

- `runExtraction`: 原本を `source/` から読み、`extracted/` に展開。
  `{ws}/{doc}` を mkdir する現行ロジックを廃止
  （[extraction.go](../../apps/worker/pkg/worker/tools/builtin/io/extraction.go)）
- `processZip`: 展開先を `extracted/` に。原本 ZIP との衝突が消える

### worker grep（検索対象を extracted/ に）

- `grepSearch` の `targetPath = DocPath(ws, doc)` を `ExtractedDir(ws, doc)` に
  （[grep.go:98](../../apps/worker/pkg/worker/tools/builtin/io/grep.go)）

### Document Map / L1 連携

- [orchestrator.prepareDocumentMap](../../apps/worker/pkg/worker/agents/orchestrator.go) の
  確定 extract は、上記 extraction の修正にそのまま乗る（追加対応不要）

### 仕様ドキュメント

- [gcs-fuse-ingestion-spec.md](../architecture/gcs-fuse-ingestion-spec.md) を本レイアウトに
  書き換え。ReadOnly マウントの記述（原本=source/ のみ ReadOnly）と整合させる

## 後方互換・移行

**後方互換は取らない。** 旧構造（`{ws}/{doc}` ファイル）の既存 job / オブジェクトは
**削除**する。理由: 旧構造は extract が必ず失敗するので価値あるデータが無く、二重サポートは
「ファイルかディレクトリか」の曖昧さを温存して再発を招く。

移行手順（実装時に詰める）:
1. 新レイアウトで API + worker をデプロイ
2. 旧 GCS オブジェクト（`{ws}/{doc}` ファイル）と対応する DB の document / job を削除
   （reset-firestore / reset_db 系スクリプトの活用）

## 決定事項（2026-06-02）

1. **ファイル命名 = 正規化名。** 単一ファイルは `source/original.{ext}`（ext は
   元ファイル名 / MIME から決定）、ZIP は `source/archive.zip` 固定。
   元のファイル名は `documents.filename`（DB）に保持済みなので表示はそこから引く。
   パスにユーザ入力（元名）を載せないことで `/` や特殊文字によるパス事故を防ぐ。
2. **API の filename 経路 = proto 変更不要。** `filename` は既に
   `CreateDocument(ctx, ws, userID, filename, …)` に渡り DB 保存されている。
   [document.go](../../apps/api/internal/repository/postgres/document.go) の
   `IssueDocumentUploadURL(ctx, ws, docID, …)` の objectName を
   `docID` → `{docID}/source/original.{ext}` に変えるだけ（ローカル組み立ての変更）。
   RPC / protobuf は触らない。
3. **ReadOnly マウントは今回やらない。** worker は `extracted/` と `.checkpoints/` に
   書くのでマウント全体を RO にはできず、`source/` だけの per-path RO は gcsfuse 上
   複雑。パス構造の統一とは独立な改善なので分離する（マウントは RW のまま）。
4. **既存データは stage を全リセット。** `manage-env.sh` の `reset-db` +
   `reset-firestore` に加え、GCS の `{ws}/` 配下を一括削除する。旧構造のデータは
   extract が必ず失敗していて価値が無く、選別削除の複雑さを負う理由がない。

## 実装メモ（2026-06-02）

実装で判明・対応した追加点:

- **worker の extract が FileURI から filename を導出していたのを廃止。**
  `runExtraction` は `source.Filename` を空で初期化し、`PopulateSourceFile` が
  `source/` の on-disk 名（`original.{ext}`）を採用する。旧コードは
  `filepath.Base(FileURI)` を使い、FileURI が `{ws}/{doc}` だと filename が
  `{document_id}` になって extracted パスと document_files が壊れる芽だった。
- **API のメタデータ取得/削除を prefix ベースに変更。** `GetObjectMetadata` /
  `DeleteDocumentObject`（本番 GCS + Fake GCS 両方）は `documentID` しか持たず ext を
  知らないので、`source/` プレフィックスを list して唯一のオブジェクトを相手にする。
  削除は `{doc}/` サブツリー全体を list して消す。
- **デッドコード `BuildDocumentObjectMetadataURL` を削除。**
- **追加テスト:** storage のパス分離 / `source/` 読み出し / on-disk 名採用、
  `Extract` の E2E（stage 回帰の経路）、`SourceObjectName` のパス・インジェクション防止。

## 残課題

- **`file_uri` の整理。** worker は FUSE マウントから読み（[load.go](../../apps/worker/pkg/worker/sourceio/load.go)
  に「HTTP fallback なし」と明記）、ディスパッチに乗る `file_uri` を実際には使わない。
  そのため `BuildDocumentSourceURL`（[bootstrap.go](../../apps/api/internal/bootstrap/bootstrap.go)）が
  旧構造 `{ws}/{doc}` を指したままでも実害はないが、将来 HTTP fallback を復活させる
  なら `source/` を指すよう直すか、`file_uri` 自体を廃止すべき。今回はスコープ外。
- **既存データの実リセット。** stage の旧構造オブジェクト + DB の document/job を削除
  （決定事項 #4）。デプロイ時に実施。

## 関連

- [gcs-fuse-ingestion-spec.md](../architecture/gcs-fuse-ingestion-spec.md) — 書き換え対象の正式仕様
- worker エージェントループ timeout 対策の L1 Document Map は本修正に依存
  （実装済み・ドキュメント削除済み、git 履歴参照）
