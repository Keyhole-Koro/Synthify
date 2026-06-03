# tree_item の kind 名を分かりやすく改名する

**Status:** 仕様ドラフト（実装前）／**[tree-node-workspace-ownership](tree-node-workspace-ownership.md) を採ると不要になる見込み**
**採用方針:** `workspace_root` / `document_root` / `node` → `workspace` / `document` / `topic`。各 kind を「その階層が何を表すか」の役割名に揃え、"root" の重複を解消する。

> **注記:** 本案は「kind 列を残して改名する」前提だが、議論の結果 [tree-node-workspace-ownership](tree-node-workspace-ownership.md)（node を workspace 直属の統合ツリーにし、`workspace_root`/`document_root`/`kind` を廃止）が採用方針となった。それが実現すると **kind 列自体が消える**ため、本改名案は実装されない可能性が高い。kind を残す判断になった場合のフォールバックとして保持する。

## やりたいこと

tree の各ノードの `kind` を、紛らわしい現状の名前から、階層の役割を直接表す名前に改名する。

```
現状                改名後
workspace_root  →  workspace
document_root   →  document
node            →  topic
```

## 現状の問題

`tree_items.kind` の 3 値（[0008_tree.up.sql:7-11](../../db/migrations/0008_tree.up.sql#L7-L11)）が紛らわしい。

1. **"root" が 2 つある** — `workspace_root` と `document_root`。木構造上の本当の根は `workspace_root` だけで、`document_root` は「部分木の頂点」にすぎない。どちらが根かパッと分からない。
2. **粒度が揃っていない** — `workspace_root` / `document_root` は役割名だが、`node` は「その他」。同じ列の値として粒度がちぐはぐ。
3. **[workspace-root-id-unification.md](workspace-root-id-unification.md) 適用後はさらに冗長** — `workspace_root` の id が workspace id と一致し「workspace そのもの」になるため、わざわざ "root" と呼ぶ意味が薄れる。

## 改名後の意味

| kind | 表すもの | 階層 |
|---|---|---|
| `workspace` | ワークスペース全体を束ねる唯一の根 | `parent_id IS NULL`、workspace に 1 個 |
| `document` | 1 つの document に対応する部分木の頂点（document との 1:1 接点） | `parent = workspace`、document ごとに 1 個 |
| `topic` | 知識構造の中身ノード（話題の入れ子） | それ以外、任意個 |

```
workspace > document > topic > topic > ...
```

「root」を一切使わず、各階層が「何を表すか」で読めるようにする。`topic` は Synthify の知識構造（話題の入れ子）として自然。

## 検討した代替案（不採用）

- **`node` を維持** — "root" 重複は消えるが、`node` だけ汎用語のまま粒度がちぐはぐに残る。**不採用**。
- **`section`** — 文書の章節イメージで document の下位として直感的だが、cross-document 概念統合後は「文書由来の区画」と限らなくなり意味がずれる。**不採用**。
- **改名せずドキュメントで補う** — リネームは proto enum / SQL CHECK 制約 / 生成コードまで 28 ファイルに及ぶため見送る案。ただし開発中の今がリネーム最安のタイミングであり、命名の負債を残す方が高くつくと判断。**不採用**。

## 影響範囲（リネーム対象）

`workspace_root` / `document_root` / `WORKSPACE_ROOT` / `DOCUMENT_ROOT` のリテラルは **約 28 ファイル**に分布。横断的で、生成コードを含む。

- **proto enum** — `contracts/connectrpc/synthify/app/v1`（`ItemKind` enum）→ `buf` 再生成（`internal/gen`, `apps/web/src/gen`）。**正バージョンの buf で生成すること**（ローカル版ずれで全ファイル diff した前例あり）。
- **SQL** — `db/migrations`（`CHECK (kind IN (...))` 制約と kind 列のコメント）、`db/queries`、`sqlcgen` 再生成。**sqlc も正バージョン（v1.31.1）で生成**。
- **backend Go** — `apps/api/internal/repository/postgres`, `mock`, `apps/api/internal/service`, `apps/worker/pkg/worker/...`。
- **frontend** — `apps/web/src/features/tree`, `apps/web/src/features/workspaces/...`（`ItemKind.WORKSPACE_ROOT` 等の参照）。
- **job 通知** — `internal/platform/job/status`（`AffectedWorkspaceRootItemID` 等の命名も追従するか検討）。

### 注意：DB 値か enum 名かでマイグレーション要否が変わる

- `kind` 列に格納される**文字列値**（`'workspace_root'` 等）を変えるなら、DB リセット（本件は開発中なのでリセット可）か migration が要る。CHECK 制約も書き換え。
- proto enum の**シンボル名**（`ITEM_KIND_WORKSPACE_ROOT`）の改名は、生成コードと参照箇所のコード変更のみ（DB 値とは独立）。
- 両方を揃えるのが望ましい（値とシンボルの乖離は新たな紛らわしさを生む）。

## 関連

- [workspace-root-id-unification.md](workspace-root-id-unification.md) — id 統一。本改名と独立だが、両方適用すると「`workspace` kind の item は id も workspace id」と一貫し、最も素直になる。順序は問わないが、まとめて入れると混乱が少ない。

## 対象外

- 木構造そのもの・1:N / 1:1 の関係（変えない）。
- cross-document tree（`item_aliases` 等）。
