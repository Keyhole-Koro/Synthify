# STIF: tree item import format

**Status:** Phase 1 実装済み
**Purpose:** Codex / Claude / Gemini などにある整理済み情報を、Synthify の workspace tree item として import できる独自ファイル形式にする。

実装の入口:

- parser / validator / sanitizer: `apps/api/internal/stif/`
- import service: `apps/api/internal/application/tree_import.go`
- RPC: `TreeImportService.ImportStif` (`contracts/connectrpc/synthify/app/v1/tree_import.proto`)
- import UI: `apps/web/src/features/stif/`（workspace paper の「インポート」ボタン）

## やりたいこと

LLM チャット、設計メモ、調査ログ、仕様書の断片などを、手作業でコピーするだけではなく、Synthify の knowledge tree に直接取り込める形にする。

必要な性質:

- 人間が読んで編集できる。
- LLM が安定して生成できる。
- tree の親子関係、rich HTML content、override CSS、出典情報を表せる。
- import 時に DB の item id を持っていなくてもよい。
- 初期実装は `append` だけで始められる。

## 形式名

**STIF: Synthify Tree Item Format**

- 拡張子: `.stif`
- MIME: `application/vnd.synthify.tree+text`
- 文字コード: UTF-8
- 改行: LF 推奨

## ファイル例

```stif
#STIF/1
@tree workspace="workspace-local-id" mode="append" scope="canonical"

::item /overview
title: "Workspace Overview"
description: "この workspace 全体の概要"
state: human_curated
cross_document: true
order: 10
---
<article>
  <h1>Overview</h1>
  <p>ここに rich HTML content を入れる。</p>
</article>
::css
article { line-height: 1.7; }
h1 { font-size: 28px; }
::end

::item /overview/api
title: "API Design"
description: "API 設計の要点"
parent: /overview
state: pending_review
source:
  - type: github
    url: https://github.com/example/synthify/blob/abc123/docs/api.md#L12-L48
    repo: example/synthify
    ref: abc123
    path: docs/api.md
    lines: [12, 48]
    confidence: 0.86
---
<p>API 設計に関する説明。</p>
::end

::item /overview/api/auth
title: "Authentication"
description: "認証まわり"
parent: /overview/api
---
<p>OAuth / session / permission model.</p>
::end
```

## 文法

### ヘッダー

先頭行は必ず以下にする。

```stif
#STIF/1
```

2 行目以降に tree metadata を置ける。

```stif
@tree workspace="workspace-local-id" mode="append" scope="canonical"
```

`workspace` は import UI / API 側で指定する場合は省略可。`mode` は初期実装では `append` のみ対応でよい。

### item block

各 node は `::item <local_path>` で始まり、`::end` で終わる。

```stif
::item /path/to/item
title: "Title"
description: "Short summary"
parent: /path/to/parent
---
<p>HTML content</p>
::end
```

`local_path` は STIF ファイル内だけで使う仮 ID。DB の `items.id` は import 時に新規採番する。`parent` は同じ STIF 内の `local_path` を参照する。

### metadata

必須:

- `title`

推奨:

- `description`
- `parent`
- `state`
- `scope`

任意:

- `cross_document`
- `order`
- `merge_key`
- `source`

`state`:

- `system_generated`
- `pending_review`
- `human_curated`
- `locked`

`scope`:

- `document`
- `canonical`

`mode`:

- `append`: 既存 tree に追加する。
- `replace`: workspace の tree を置き換える。将来対応。
- `merge`: `merge_key` / `local_path` / `title` で既存 item に統合する。将来対応。

### source

`source` は item の根拠を表す。単なるメモではなく、Synthify 側が後から interactive に原文取得、preview、再要約、差分確認をするための reference handle として扱う。

```stif
source:
  - type: github
    url: https://github.com/owner/repo/blob/abc123/docs/spec.md#L10-L42
    repo: owner/repo
    ref: abc123
    path: docs/spec.md
    lines: [10, 42]
    confidence: 0.92
```

`url` は人間が確認しやすい入口として残す。import / retrieval の機械処理では `type` と構造化 field を優先する。

source type:

- `github`: GitHub repository の file / issue / PR / comment。
- `web`: 一般 Web URL。
- `local_file`: import 実行環境または repository 内の file path。
- `chat`: Codex / Claude / Gemini などの会話 transcript。
- `document`: Synthify 内または外部 document store の document / chunk。

GitHub source fields:

- `repo`: `owner/repo`。
- `ref`: commit SHA / branch / tag。再現性が必要な根拠は commit SHA を推奨する。
- `path`: repository 内 path。file source では必須。
- `lines`: `[start, end]`。line range がある場合だけ付ける。
- `url`: GitHub の canonical URL。人間向け fallback。
- `kind`: `file` / `issue` / `pull_request` / `comment`。省略時は `file`。
- `id`: issue number / PR number / comment id など。`kind` が file 以外の場合に使う。
- `confidence`: 0.0-1.0。source と item 内容の対応度。

interactive retrieval の方針:

- import 時点では source handle を保存し、原文取得は UI 操作時に遅延実行してよい。
- private repository や認証が必要な source は、import 実行者の接続済み account / token で取得する。
- branch ref は内容が変わるため、取得時に snapshot commit SHA と content hash を保存できるとよい。
- commit SHA 固定 source は再現性を優先し、branch source は最新追従を優先する。
- 取得した本文を item content に自動混入しない。preview / 再要約 / 子 item 化などの明示操作を経由する。

### content

`---` から `::css` または `::end` までを `content` として扱う。HTML fragment を想定する。

```stif
---
<section>
  <h2>Key Points</h2>
  <ul>
    <li>...</li>
  </ul>
</section>
```

Markdown を入れたい場合も、LLM 変換段階で HTML に寄せる。import 側で Markdown parser を持たないほうが実装が単純になる。

### override CSS

`::css` から `::end` までを `override_css` として扱う。

```stif
::css
.callout {
  border-left: 4px solid #3b82f6;
  padding-left: 12px;
}
::end
```

CSS は iframe 内で隔離表示される前提。初期実装では危険な CSS / URL の扱いを import sanitize の対象にする。

## 既存モデルへのマッピング

| STIF | backend domain / proto |
| --- | --- |
| `local_path` | import 中の仮 ID |
| `title` | `Item.title` |
| `description` | `Item.description` |
| content block | `Item.content` |
| `::css` block | `Item.override_css` |
| `parent` | import 時に `Item.parent_id` へ解決 |
| child order | import 時に `Item.child_ids` / DB ordering へ反映 |
| `state` | `Item.governance_state` |
| `scope` | `Item.scope` |
| `cross_document` | `Item.cross_document` |
| `source` | `ItemSource` |

## import 方針

Phase 1 (**実装済み**):

- `.stif` を parse する。
- `#STIF/1` と item block の構文検証をする。
- `append` のみ対応する。`replace` / `merge` は error diagnostic で拒否する。
- `local_path` の重複を拒否する。
- `parent` が存在しない場合は reject する。
- cycle を reject する。
- item id は import 時に生成する。
- `created_by` は import 実行者にする。
- `state` 省略時は `human_curated` にする。
- `scope` 省略時は `canonical` にする。
- `source` は parse / validate して `item_source_refs` に保存する。外部取得は import の必須処理にしない。
- HTML / CSS を allowlist で sanitize する。
- import preview (dry run) を返す。

Phase 2:

- `merge_key` による既存 item 更新。
- `replace` mode。
- GitHub source の interactive preview / open in GitHub。
- GitHub source の snapshot commit SHA / content hash 保存。
- export to STIF。

## Phase 1 の実装仕様

### エラー方針

parse / validation の問題は RPC error ではなく **diagnostic** として返す。import UI は 1 回の往復で全ての問題を出したいので、最初の 1 件しか運べない error code には載せない。

- `ERROR` が 1 件でもあれば **何も書き込まない**。部分 import は親が居ない item を作るので、全部入るか全部入らないかのどちらかにする。
- `WARNING` は import を止めない。sanitize による除去、未知の metadata key、`merge_key` の無効化などが該当する。

### preview は server 側 dry run

`ImportStif` の `dry_run` で parse / validate / sanitize だけ行い、書き込まない。parser を web 側にも置くと preview と commit が食い違いうるので、parser は Go 側 1 本にした（未決定事項の 1 つ目への回答）。

commit 時にも server は再 parse する。preview を通っていない document や、preview 後に書き換えられた document は、commit 側で改めて拒否される。

### import API の置き場所

`ItemService` に足さず `TreeImportService` を新設した（未決定事項の 2 つ目への回答）。import は今後 `replace` / `merge` mode や export to STIF など独自の語彙を持つ想定で、単一 item の CRUD とは別物のため。

認証は既定どおり必須。share token では叩けない（書き込み RPC のため）。`workspace_id` に対する editor / owner 権限を要求し、dry run も同じ権限を要求する。

### `order` の保存方法

`tree_items` に order 列は足していない（未決定事項の 4 つ目への回答）。兄弟の順序は `created_at ASC` で決まるので、parser が items を「親が先・兄弟は `order` 昇順（同値は文書順）」に並べ替えて返し、importer がその順に insert する。`order` 未指定は 0 として扱う。

### `scope` は永続化されない

`tree_items` に scope 列がないため、`scope` は parse / 検証はするが保存されない。preview の応答には含まれる。scope を実際に効かせるにはスキーマ変更が要る。

### sanitize policy

（未決定事項の 3 つ目への回答）content は iframe 内に描画されるが、その iframe は自前の bootstrap script を動かすので、iframe 境界だけでは足りない。**保存時**に sanitize する（描画時ではない）ので、preview が見せたものと DB に入るものが同一になる。

HTML:

- `golang.org/x/net/html` で parse し、allowlist で filter して **再 render** する。文字列置換ではないので、壊れた markup でタグを密輸できない。
- `script` / `style` / `iframe` / `object` / `embed` / `form` / `svg` などは subtree ごと削除する。
- allowlist 外のその他の要素は unwrap（中身のテキストは残す）。
- `on*` 属性は全て削除する。
- `href` / `src` / `cite` は `http` / `https` / `mailto` / 相対 / `data:image/*` のみ許可する。`javascript:` `vbscript:` `data:text/html` は削除する。
- `data-*` / `aria-*` は保持する。paper renderer が `data-paper-id`（子 paper へのリンク）と `data-file-id`（画像の遅延読み込み）を使うため。

CSS（`::css` block と `style` 属性の両方）:

- `@import` / `expression(` / `behavior` / `-moz-binding` / `javascript:` を含む宣言・at-rule を落とす。落とすのは宣言単位なので、1 行が原因で stylesheet 全体を失わない。
- `url()` は `data:image/*` と `https:` のみ許可する。remote `url()` は rule が match した瞬間に request を出す side channel になるため。

### `source` の parse

独自 parser で扱っている（未決定事項の 5 つ目への回答）。metadata 部分だけ YAML parser に渡す構成は取らなかった。`type` は必須で、省略すると error diagnostic になる（`item_source_refs.source_type` の CHECK 制約と同じ集合）。

### parser の寛容さ

実運用で LLM 出力をそのまま貼れるように、以下は受け入れる:

- 先頭の BOM、`#STIF/1` の前の空行。
- 文書全体を包む Markdown code fence（```` ```stif ````）。
- CRLF 改行。
- `parent` 省略時は `local_path` の親パスから推測する（`/overview/api` → `/overview`）。推測先が文書内に無ければ root item として import し、warning を出す。

### 制限

1 回の import の `source` は 1 MiB まで。

## 未決定事項（Phase 1 で解消されなかったもの）

- `order` を DB 列として持つべきか。現状は insert 順に畳み込んでいるので、import 後に item を並べ替える UI が来ると再考が要る。
- `scope` を `tree_items` に持たせるか。
- GitHub source retrieval を backend 経由にするか、web 側 connector 経由にするか。
- private repository の token scope / permission model。
- STIF JSON 表現（Phase 3）。

Phase 3:

- source evidence の完全取り込み。
- source からの再要約 / 子 item 生成。
- source 更新差分の検出。
- conflict resolution。
- STIF JSON 表現。

## LLM 変換プロンプト

Codex / Claude / Gemini などにある情報を STIF にする時は、以下のプロンプトを使う。

```text
あなたは Synthify の tree item import 用フォーマッタです。
以下の入力情報を STIF/1 に変換してください。

目的:
- 入力情報を Synthify の knowledge tree として import できる `.stif` にする。
- 重要な概念を tree item に分割し、親子関係を作る。
- 各 item は title, description, HTML content を持つ。

出力ルール:
- 出力は STIF だけにしてください。説明文や Markdown fence は不要です。
- 先頭行は必ず `#STIF/1` にしてください。
- 2 行目に `@tree mode="append" scope="canonical"` を入れてください。
- 各 item は `::item /stable/path` で始め、`::end` で終えてください。
- `title` は必須です。
- `description` は 1-2 文で要約してください。
- `parent` は root 以外に必ず付けてください。
- content は `---` の後に HTML fragment として書いてください。
- Markdown ではなく HTML を使ってください。
- HTML は article, section, h1-h3, p, ul, ol, li, table, code, pre, blockquote を中心にしてください。
- script, iframe, external image, external stylesheet は使わないでください。
- CSS が必要な場合だけ `::css` block を追加してください。
- 事実と推測を混ぜないでください。推測は content 内で「推測」と明記してください。
- 入力にない出典 ID は作らないでください。
- item 数は情報量に応じて 3-12 個程度にしてください。

tree 設計ルール:
- 最初の item は `/overview` にしてください。
- `/overview` は全体概要です。
- 主要テーマは `/overview/<topic>` の下に置いてください。
- 詳細トピックは `/overview/<topic>/<detail>` に置いてください。
- local_path は lowercase kebab-case にしてください。
- 同じ概念を重複 item にしないでください。

metadata:
- state は `human_curated` にしてください。
- 複数の文書や会話を統合した item は `cross_document: true` にしてください。
- 順序が重要な場合は `order` を 10, 20, 30... のように付けてください。
- 入力に GitHub URL / file path / document id / chat id などの根拠がある場合は `source` に残してください。
- GitHub source は URL だけでなく、可能な限り `type`, `repo`, `ref`, `path`, `lines` に分解してください。
- branch URL より commit SHA が入力にある場合は commit SHA を `ref` に使ってください。
- 入力にない repository、commit、path、line number、出典 ID は作らないでください。

source 例:
source:
  - type: github
    url: https://github.com/owner/repo/blob/abc123/docs/spec.md#L10-L42
    repo: owner/repo
    ref: abc123
    path: docs/spec.md
    lines: [10, 42]
    confidence: 0.92

入力:
<<<
ここに Codex / Claude / Gemini / メモ / 仕様書の内容を貼る
>>>
```

## Claude / Gemini 向け補足

長い会話を変換する場合は、先に「決定事項」「未決定事項」「実装タスク」「背景知識」に分類させてから STIF にするほうが安定する。

```text
まず入力を以下に分類し、その分類結果だけを使って STIF/1 を生成してください。

- Decisions: 決定済みの仕様・方針
- OpenQuestions: 未決定事項
- Tasks: 実装タスク
- Context: 背景説明
- Risks: リスク・注意点

分類結果そのものは出力せず、最終出力は STIF だけにしてください。
```

## Codex 向け補足

リポジトリ内の情報から STIF を作る場合は、対象ファイルのパスを source に残す。

```text
リポジトリ内の設計情報を STIF/1 に変換してください。
各 item の content には、根拠になるファイルパスを `<p><strong>Sources:</strong> ...</p>` として末尾に入れてください。
各 item の metadata には、根拠になるファイルパスを `source` として構造化して残してください。
GitHub repository の remote URL と commit SHA が分かる場合は、`type: github`, `repo`, `ref`, `path`, `lines` を使ってください。
GitHub 情報が分からない場合は、`type: local_file`, `path` を使ってください。
ファイルパス、URL、line number は入力に含まれるものだけを使い、存在しない値は作らないでください。
```

（旧「未決定事項」節は Phase 1 で解消済み。回答は上の「Phase 1 の実装仕様」、残件は「未決定事項（Phase 1 で解消されなかったもの）」を参照。）
