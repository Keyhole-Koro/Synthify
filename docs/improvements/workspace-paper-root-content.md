# ワークスペース paper に唯一 root node の content を表示する

**Status:** 設計ドラフト（実装前）
**前提:** [tree-node-workspace-ownership](tree-node-workspace-ownership.md)（node 直属モデル）が実装済み。本設計はその上に「ワークスペース＝1つの統合知識レポート」という UI を載せる。

## やりたいこと

ワークスペース paper を開いたら、そのワークスペースの **唯一の root node が持つ `content`（LLM 生成のリッチ HTML + `override_css`）を iframe で表示**する。ワークスペースが「1つの読めるレポート」に見える形にする。

```
現状:                          あるべき姿:
WorkspacePaper content:        WorkspacePaper content:
┌ Header                       ┌ Header（名前・状態・ピン・アップロード）
├ root node の card 一覧 ★     ├ root node の content を iframe 表示 ★
├ 最近のジョブ                  │   （リッチ HTML + override_css、CSS 隔離）
└ アップロード欄                └ 最近のジョブ / アップロード欄（補助）
```

★ が変更点。「root node を開くための card」をやめ、「root node の中身そのもの」を見せる。

## 背景：node 直属モデル後の paper の意味

[node 直属モデル](tree-node-workspace-ownership.md)で、ワークスペースの知識は workspace 直属の1本の統合ツリーになった。`childItems`（旧 document_root card）は今や「knowledge node」を指すが、UI はまだ「ドキュメント card を並べる」古い見た目のまま（`WorkspaceDocumentList`）。document は出典になり、card で並べる対象ではなくなった。

→ ワークスペース paper は「文書置き場」ではなく「**統合された知識レポートの表紙**」であるべき。その表紙の中身が root node の content。

## 設計（採用）

### データモデル：唯一 root node を保証する

- ワークスペースのツリーは **常に唯一の root node**（`parent_id IS NULL`）を持つ。
- その root node の `content` が「ワークスペース全体の概要レポート」。
- 配下に knowledge node がぶら下がる。

```
workspace（ツリーの根、DB実体なし）
└─ root node（唯一・parent_id=NULL・content=トップ概要HTML）  ← iframe 表示
    ├─ node（document A 由来 / 統合）
    ├─ node（document B 由来 / 統合）
    └─ ...
```

**唯一 root の保証（確定方針）:**
- **初回 document 処理時**、LLM がトップ概念を root node として生成する（content も生成）。
- **2回目以降の新規 node は必ず既存 root node の子（またはその子孫）として作る**。workspace 直下（parent 空）に新たな root を作らない。統合（merge）はブロックB のロジックどおり既存 node を更新。
- 結果、root node は常に 1 つ。LLM は「新概念をツリーのどこにぶら下げるか」を常に既存ツリーから選ぶ。

> 注: これは旧 `workspace_root` と構造は似るが役割が逆。旧 root は「UI で隠す空の枠」、新 root は「content を持つ主役（ワークスペースの表紙）」。

### UI：root content を iframe 表示

- paper-in-paper には既に **`PaperContentFrame`** があり、`content`(HTML string) + `overrideCss` を **iframe で CSS 隔離描画**する（[PaperContentFrame.tsx:55](../../apps/web/vender/paper-in-paper/src/lib/react/components/PaperContentFrame.tsx#L55)）。基盤は流用できる。
- ワークスペース paper の content に、root node の `content` を表示する領域を設ける。CSS 隔離が iframe を使う主目的（`override_css` を親ページと衝突させない）。
- アップロード欄・ジョブ一覧・リネームなどの操作 UI（現 `WorkspacePaper`）は **iframe の外（chrome 部分）**に残す。iframe は「読む中身」、その周りが「操作」。

### 想定レイアウト

```
┌─ WorkspaceHeader（名前・docs数・状態・ピン・rename）
├─────────────────────────────────────────────
│  ╔═══ iframe ═══════════════════════════╗
│  ║ root node.content                     ║   ← リッチHTML + override_css
│  ║  <div class="hero-block">…</div>      ║      CSS隔離
│  ║  …                                    ║
│  ╚═══════════════════════════════════════╝
├─────────────────────────────────────────────
├─ 最近のジョブ（常時・補助）
└─ アップロード欄（ドロップゾーン・補助）
```

## 影響範囲

### backend / worker — 唯一 root の保証
- [persistence.go](../../apps/worker/pkg/worker/tools/builtin/io/persistence.go): 新規 node の親決定を変更。既存 root が無ければ最初の top-level item を root に、以降の top-level item は root の子に強制。既存 root があれば top-level item は root の子に。
- 生成プロンプト（[knowledge_tree.system.tmpl](../../apps/worker/pkg/worker/prompts/templates/knowledge_tree.system.tmpl)）: 「ワークスペースには唯一の root がある。新概念は既存ツリーのいずれかの node にぶら下げよ（merge_target か parent を必ず既存 node に）」を追加。root content は処理のたびに改稿してよい（merge ロジックで root 自身を merge 対象にできる）。

### frontend — projection と WorkspacePaper
- [useWorkspaceProjection.ts](../../apps/web/src/features/workspaces/useWorkspaceProjection.ts): 唯一 root を特定し、その content をワークスペース paper の content につなぐ（または WorkspacePaper 内に iframe 領域として埋める）。
- [WorkspacePaper.tsx](../../apps/web/src/features/workspaces/paper/WorkspacePaper.tsx): `WorkspaceDocumentList`（root node card 一覧）を撤去し、root content の iframe 表示に置換。Header / ジョブ一覧 / アップロード欄は chrome として残す。
- `WorkspaceDocumentList` は役割を失うため削除 or 「配下 node へのナビ」に転用を検討。

### paper-in-paper（vendored）
- 既存 `PaperContentFrame` の iframe + overrideCss 描画を流用できるか、ワークスペース paper 専用に薄いラッパが要るか実装時に判断。vendored lib の改変は最小に。

## 未決定事項（実装前に詰める）

1. **iframe の高さ/スクロール** — root content は長くなりうる。固定高 + 内部スクロールか、paper の高さに追従か。
2. **配下 node へのナビゲーション** — root content 内の `data-paper-id` リンク（既存の content-link 機構）で子 node を開く。card 一覧を別途出すか、content 内リンクだけにするか。
3. **空状態** — まだ document 未処理で root が無いワークスペースの表示（現 `WorkspaceEmptyHeader` + ドロップゾーン）。
4. **root content 改稿のタイミング** — 新 document 統合のたびに root content を LLM が書き直すか、初回のみか。書き直すなら root を毎回 merge 対象に含める。
5. **cross_document の可視化** — 統合 node のバッジ等（[tree-node-workspace-ownership](tree-node-workspace-ownership.md) の cross_document を UI に出すか）。

## 関連

- [tree-node-workspace-ownership.md](tree-node-workspace-ownership.md) — node 直属モデル + 統合ロジック（本設計の前提）。
- 旧 [workspace-content-embedded-tree.md] は「card を並べる」案だったが削除済み。本設計は「content を見せる」方向に転換。

## 対象外

- ツリー全体のグラフ可視化。
- root content のインライン編集（閲覧のみ。編集は別テーマ）。
