# 15. Frontend Specification

## Overview

フロントエンドは `TypeScript + React` で実装し、`Firebase Hosting` から配信する。

グラフの可視化には `@keyhole-koro/paper-in-paper` を使用する。各ノードを `Paper` としてツリー構造で表示し、ユーザーが能動的にスコープを広げながら知識を探索できるようにする。`is_dev=true` のメンバーのみ `/dev/stats` にアクセスできる。

---

## UI State Naming Policy

フロントの状態名は [data-model.md](../domain/data-model.md) の state family に準拠し、UI 固有の family を追加する。

### Frontend State Families

| Family | 用途 | 主な値 |
| --- | --- | --- |
| `DocumentLifecycleState` | ドキュメント一覧・バッジ・再処理ボタンの表示制御 | `uploaded` / `pending_normalization` / `processing` / `completed` / `failed` |
| `GraphProjectionScope` | node が document graph か canonical graph かの区別 | `document` / `canonical` |
| `PathSearchMode` | 経路検索 UI の操作状態 | `inactive` / `picking_source` / `picking_target` / `results` |
| `MetaPanelState` | メタデータパネルの表示状態 | `closed` / `open` |

### Naming Rules

- backend 由来の `status` は family 名を付けて `documentStatus` のように明示する
- `scope` は `GraphProjectionScope` 専用語とし、単純な UI タブ切り替えには使わない
- paper-in-paper の内部状態（`expansionMap`, `focusedNodeId`, `importanceMap` 等）は library の型をそのまま使う

---

## ルート構成

| パス | 用途 | 備考 |
| --- | --- | --- |
| `/` | ワークスペース一覧または自動リダイレクト | ワークスペースが 1 つの場合は `/w/:workspaceId` へリダイレクト |
| `/w/:workspaceId` | ワークスペースホーム（ドキュメント管理・アップロード） | |
| `/w/:workspaceId/explore` | ペーパーエクスプローラ（ドキュメント横断ビュー） | |
| `/w/:workspaceId/settings` | ワークスペース設定（メンバー管理・権限・プラン） | `owner` は全操作可、`editor` は閲覧中心 |
| `/dev/stats` | 開発者向け統計ビューワー（`is_dev=true` のみ） | |

### URL 状態管理

| パラメータ | 型 | 説明 |
| --- | --- | --- |
| `doc` | string | フォーカス中のドキュメント ID |
| `node` | string | フォーカス中のノード ID（paper-in-paper の `focusedNodeId`） |
| `path_src` | string | 経路検索の起点ノード ID |
| `path_dst` | string | 経路検索の終点ノード ID |

---

## 画面構成

### グローバルヘッダ

```
┌──────────────────────────────────────────────────────────────────────┐
│  [≡]  Synthify  [ws ▾]  root / 販売戦略 / テレアポ施策  [⌘K]  [👤▾] │
└──────────────────────────────────────────────────────────────────────┘
```

- `[≡]` : サイドバー開閉トグル
- `[ws ▾]` : ワークスペース切り替えドロップダウン
- パンくず部分: paper-in-paper の `Breadcrumbs` コンポーネント（`focusedNodeId` から root までの祖先列）
- `[⌘K]` : コマンドパレット起動
- `[👤▾]` : ユーザーメニュー（プロフィール・ログアウト・プラン）

パンくずの挙動は paper-in-paper の仕様に従う：クリックでそのノード以下のブランチを閉じ、`focusedNodeId` を更新する。

---

## ワークスペースホーム（`/w/:workspaceId`）

```
┌──────────────────────────────────────────────────────────────────────┐
│  Header                                                              │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ドキュメント   [全て ▾]  [完了 ▾]  [+ アップロード]   🔍 検索      │
│  ──────────────────────────────────────────────────────────────────  │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐         │
│  │ 📄 report.pdf  │  │ 📄 strategy.md │  │ 📄 data.csv    │         │
│  │                │  │                │  │                │         │
│  │ ✅ 完了        │  │ ⏳ 処理中      │  │ 🛠 承認待ち    │         │
│  │ 136 nodes      │  │ pass1 / 3分    │  │ 正規化レビュー │         │
│  │ [開いて探索]   │  │ [ログ]         │  │ [詳細][再開]   │         │
│  └────────────────┘  └────────────────┘  └────────────────┘         │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### ドキュメントカードのステータスバッジ

| `DocumentLifecycleState` | バッジ表示 | カード上のアクション |
| --- | --- | --- |
| `uploaded` | ⬆ アップロード済 | [処理開始] |
| `pending_normalization` | 🛠 承認待ち | [詳細][再開]（`dev`/`editor` のみ） |
| `processing` | ⏳ 処理中（スピナー + ステージ名） | [ログ] |
| `completed` | ✅ 完了（ノード数表示） | [開いて探索] |
| `failed` | ❌ 失敗（エラー概要） | [ログ][再処理] |

- `processing` 時はステージ名（例: `pass1_extraction`）をリアルタイムで表示する
- `pending_normalization` の [再開] は `ResumeProcessing` RPC を呼ぶ
- `failed` の [再処理] は `StartProcessing(force_reprocess=true)` を呼ぶ
- zip ファイルの場合はカードを展開して内部ファイルごとのステータスを確認できる

### アップロード UI

```
┌────────────────────────────────────────────────┐
│                                                 │
│   ドラッグ＆ドロップ または クリック            │
│                                                 │
│   対応: PDF / Markdown / TXT / CSV / zip        │
│                                                 │
│   抽出粒度:  ○ full（level 0-3）  ○ summary（level 0-2）  │
│                                                 │
└────────────────────────────────────────────────┘
```

- アップロード後、確認ダイアログなしで `CreateDocument` → `StartProcessing` を即時呼び出す
- `extraction_depth` はトグルで選択（デフォルト: `full`）
- `free` プランの場合 `summary` のみ選択可能で `full` はグレーアウト

---

## ペーパーエクスプローラ（`/w/:workspaceId/explore`）

### paper-in-paper へのデータマッピング

抽出済みノードを `Paper` としてマッピングし、`PaperCanvas` で表示する。

| ノードフィールド | Paper フィールド | 説明 |
| --- | --- | --- |
| `node_id` | `Paper.id` | ペーパー識別子 |
| `label` | `Paper.title` | ヘッダに表示されるタイトル |
| `description` | `Paper.description` | コンパクトカードのホバー時に表示 |
| `summary_html` | `Paper.content` | iframe で描画されるコンテンツ |
| `summary_html` 内の構造化リンクとアプリ側 tree 解決 | `Paper.childIds` / `Paper.parentId` | ツリー構造 |

`summary_html` が null のノードは `description` をプレーンテキストでコンテンツとして使う。

### 関連ノードリンクの表現

ノード間の関連は、`html_summary_generation` ステージで生成する `summary_html` 内の `data-paper-id` リンクとして埋め込む。

```html
<!-- related node link example -->
<p>この施策は <a data-paper-id="nd_xxx">CV率3.2%</a> を根拠とする。</p>

<!-- another related node link example -->
<p>一方で <a data-paper-id="nd_yyy">テレアポは関係構築に強み</a> という反論もある。</p>
```

iframe 内でリンクをクリックすると `postMessage` 経由で `OPEN_NODE` が発火し、対象ペーパーが展開される。

### ノードアクティビティ表示

各 paper のヘッダ右上に、最近そのノードを閲覧したメンバーと手動追加者を表示する。

```
┌──────────────────────────────────────────────┐
│ 販売戦略                              [AK][MN] │
│ viewed by 2 • added by Aki                  │
└──────────────────────────────────────────────┘
```

- `OPEN_NODE` 成功時に `RecordNodeView(workspace_id, node_id, document_id)` を fire-and-forget で送る
- focused 中ノードについては hover または panel open 時に `GetUserNodeActivity` を使って閲覧者詳細を取得する
- `created_by != null` のノードは `Added by {display_name}` バッジを表示する
- 自分が未読のノードかどうかを `viewedNodeIds` で判定し、未読ノードはヘッダに subtle dot を表示する

### 手動ノード追加

`editor` / `owner` は sidebar の `[+ New]` から手動ノードを追加できる。

- 追加フォームでは `label`, `category`, `level`, `description`, `parent_node_id` を入力する
- 保存時に `CreateNode` RPC を呼び、成功後に `paperMap` へ挿入する
- `parent_node_id` が指定された場合は新ノードをその room に即時表示する
- `viewer` には `[+ New]` を表示しない

### レイアウト

```
┌──────────────────────────────────────────────────────────────────────┐
│ Header（breadcrumbs）                                                │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────────────────────────────────┐  ┌─────────────────┐  │
│  │ 事業戦略（root / level 0）               │  │  未配置ノード   │  │
│  │ ─────────────────────────────────────── │  │ ─────────────── │  │
│  │ <summary_html>                          │  │  [Claim A]      │  │
│  │                                          │  │  [Evidence B]   │  │
│  │  ┌──────────────────┐  ┌─────────────┐  │  │  [+ 新規]       │  │
│  │  │ 販売戦略（展開中）│  │ マーケ戦略  │  │  └─────────────────┘  │
│  │  │                  │  │（コンパクト）│  │                       │
│  │  │ <summary_html>   │  └─────────────┘  │                       │
│  │  │                  │                   │                       │
│  │  │  ┌────────────┐  │                   │                       │
│  │  │  │ テレアポ施策│  │                   │                       │
│  │  │  └────────────┘  │                   │                       │
│  │  └──────────────────┘                   │                       │
│  └──────────────────────────────────────────┘                       │
└──────────────────────────────────────────────────────────────────────┘
```

### 展開モデル

paper-in-paper の仕様（[behavior-spec.md](../../../../vender/paper-in-paper/docs/behavior-spec.md)）に従う。

- コンパクトカードをクリック → `OPEN_NODE` → 対象ペーパーが展開される（`focusedNodeId` も更新）
- `summary_html` 内の `data-paper-id` リンクをクリック → `OPEN_NODE` → 対象ペーパーが展開される
- 展開中ペーパーの閉じるボタン → `CLOSE_NODE` → そのサブツリーの展開状態が消去される
- スペース圧迫時は重要度の低いペーパーが自動縮小される（`AUTO_CLOSE_NODE`）
- ユーザーが手動で展開したノードは一定期間 protect される

### サイドバー（未配置ノード）

paper-in-paper の `Sidebar` コンポーネントを使う。以下のノードを `unplacedNodeIds` として扱う：

- tree に配置されていない孤立ノード（`claim` / `evidence` のみのノード等）
- ドキュメント横断で出現する canonical alias ノードのうち、ツリー構造に収まらないもの

サイドバーのカードをドラッグして任意の部屋に配置できる。配置されたノードは `ATTACH_UNPLACED_NODE` で tree に追加される。

### ドキュメント切り替え

左サイドバー（ワークスペース内ドキュメント一覧）からドキュメントを選ぶと、選択 document のノードツリーを `GetGraph` で取得し `paperMap` を更新する。

複数ドキュメントを同時選択すると、`node_aliases` で解決した canonical ノードを root に持つ統合ツリーを構築して表示する。

### 右サイドアクティビティパネル

メタデータパネルとは別に、選択ノードのユーザーアクティビティを表示する簡易パネルを開ける。

- `recent viewers`: `GetUserNodeActivity` の `viewed_nodes` から該当 node を持つメンバーを集約
- `added by`: `created_by` を解決して表示
- `same user recent nodes`: 選択ノードの追加者または最近の閲覧者が直近に見ていたノードを表示

---

## メタデータパネル

ペーパーヘッダをクリックすると右側からスライドインするパネル。paper-in-paper の `content` 領域に収まらない補足情報を表示する。

```
┌──────────────────────────────────────────────────┐
│ [×]  concept / level 2                           │
│                                                  │
│  テレアポ施策                                    │
│  ────────────────────────────────────────────── │
│  出典                                            │
│  [report.pdf / p.3]                              │
│  "テレアポ施策として月次100件を目標に..."         │
│                                                  │
│  関連ノード                                      │
│  ↑ 販売戦略                                      │
│  → CV率3.2%                                      │
│  ↙ スクリプト改善                                │
│                                                  │
│  [経路検索の起点にする]  [経路検索の終点にする]  │
└──────────────────────────────────────────────────┘
```

- `GetGraphEntityDetail` を呼び出してメタデータを取得する（パネルを開いた時のみ）
- 出典には `source_filename` とページ番号を表示する
- 関連ノードには `summary_html` に含まれていない補助リンク候補も含めて表示できる
- `scope=canonical` のノードは `representative_nodes` と canonical ラベルを表示する

---

## コマンドパレット（⌘K）

どの画面からも `⌘K`（Windows: `Ctrl+K`）で起動するオーバーレイ検索。

```
┌──────────────────────────────────────────────────────┐
│  🔍  ノード・ドキュメントを検索...                   │
├──────────────────────────────────────────────────────┤
│  最近使用                                            │
│  📄  report.pdf                    ドキュメントを開く│
│  ●   販売戦略              ペーパーにフォーカス      │
│                                                      │
│  アクション                                          │
│  ⬆  新しいファイルをアップロード                    │
│  ⇌  経路検索モードを開始                            │
└──────────────────────────────────────────────────────┘
```

- ノードラベル・ドキュメント名・アクションを横断検索する
- ノード選択でそのペーパーを `OPEN_NODE` して `focusedNodeId` を更新する

---

## 経路検索

`FindPaths` RPC で取得した経路を paper-in-paper の展開操作に変換して表示する。

### モード遷移

```
inactive → [⇌ Path] → picking_source → (ペーパー選択) → picking_target
         → (ペーパー選択) → [検索実行] → results
```

### 検索パラメータバー

```
╭────────────────────────────────────────────────────────╮
│ 経路検索  起点: [販売戦略]  終点: [CV率3.2%]           │
│ max depth: [4▾]  [cross-doc: ○]  [検索] [キャンセル]   │
╰────────────────────────────────────────────────────────╯
```

### Path Results Tray（画面下部にスライドアップ）

```
┌──────────────────────────────────────────────────────────────────────┐
│ 3 件の経路が見つかりました                           [×] 閉じる     │
├──────────────────────────────────────────────────────────────────────┤
│ ● Path 1  販売戦略 → SNS施策 → CV率3.2%    2 hop  📄 doc_001         │
│   Path 2  販売戦略 → テレアポ → A社事例 …  3 hop  📄 doc_001         │
│   Path 3  販売戦略 → マーケ戦略 → SNS → … 4 hop                     │
└──────────────────────────────────────────────────────────────────────┘
```

経路を選択すると、その経路上の全ペーパーを順に `OPEN_NODE` して展開し、終点ノードを `focusedNodeId` に設定する。

---

## フロント状態設計

### paper-in-paper 管理の状態

paper-in-paper の `PaperViewState` をそのまま使う。

```ts
interface PaperViewState {
  paperMap: PaperMap;
  expansionMap: ExpansionMap;
  unplacedNodeIds: PaperId[];
  focusedNodeId: PaperId | null;
  accessMap: AccessMap;
  importanceMap: ImportanceMap;
  manualPlacementMap: PlacementMap;
  contentHeightMap: Map<PaperId, number>;
  protectedUntilMap: Map<PaperId, number>;
}
```

### Synthify 固有の状態

- `selectedDocumentIds` : 現在表示中のドキュメント ID 群
- `pathSearchMode` : `PathSearchMode`
- `pathSearchDraft` : 経路検索の `source_node_id` / `target_node_id`
- `pathResults` : `FindPaths` のレスポンス
- `selectedPathIndex` : Path Results Tray で選択中の経路インデックス
- `metaPanelState` : `MetaPanelState`
- `metaPanelNodeId` : メタデータパネルで表示中のノード ID
- `viewedNodeIds` : 現在ユーザーが既読の node ID 集合
- `nodePresenceMap` : `node_id -> recent viewers / creator` の軽量表示データ
- `workspaceMembers` : workspace settings と avatar 表示に使うメンバー一覧
- `workspaceRole` : 現在ユーザーの `owner` / `editor` / `viewer`
- `isDevMember` : `/dev/stats` 表示判定

### 更新ルール

- ドキュメントを切り替えた際は `GetGraph` を呼び、`paperMap` を更新する（展開状態はリセット）
- 複数ドキュメント選択時は canonical ノードを root とした統合 `paperMap` を構築する
- `OPEN_NODE` はペーパーコンテンツのリンククリック・コンパクトカードクリック・コマンドパレットからのジャンプで発火する
- `OPEN_NODE` の後に非同期で `RecordNodeView` を送信し、成功時は `viewedNodeIds` と `nodePresenceMap` を更新する
- 経路検索で経路選択後は、経路上の全ノードに対して `OPEN_NODE` を順に dispatch する
- `CreateNode` 成功時は `paperMap` に node を挿入し、必要なら親の `childIds` も更新する
- workspace 切り替え時は `GetWorkspace` を呼び、`workspaceMembers`, `workspaceRole`, `isDevMember` を更新する

---

## ワークスペース設定（`/w/:workspaceId/settings`）

owner / editor / viewer の管理と invite flow を扱う。

```
┌──────────────────────────────────────────────────────────────────────┐
│ Workspace Settings                                                   │
├──────────────────────────────────────────────────────────────────────┤
│ Members                                                              │
│  Aki Tanaka      owner   dev  [Transfer ownership]                   │
│  Mina Sato       editor  -    [Role ▼] [Dev toggle] [Remove]         │
│  Ken Ito         viewer  -    [Role ▼] [Dev toggle] [Remove]         │
│                                                                      │
│ Invite member                                                        │
│  [email________________] [viewer ▼] [dev □] [Invite]                │
└──────────────────────────────────────────────────────────────────────┘
```

### 権限

- `owner` は invite, role change, remove, ownership transfer, plan 管理が可能
- `editor` は member 一覧閲覧と invite が可能だが、role change / remove / transfer ownership は不可
- `viewer` は設定画面を read-only で閲覧できるが変更操作は不可

### Invite Flow

- 招待フォーム送信で `InviteMember` を呼ぶ
- 既存ユーザーなら即時に member 一覧へ追加する
- 未登録メールアドレスなら `pending invite` 行として表示し、初回ログイン後に確定させる
- `owner` ロールは UI から選べない

### Ownership Transfer

- `owner` は別メンバーに対して `TransferOwnership` を実行できる
- 実行前に確認モーダルを表示する
- 移譲後、旧 owner は `editor` へ降格する

---

## キーボードショートカット

| ショートカット | 動作 |
| --- | --- |
| `⌘K` / `Ctrl+K` | コマンドパレット |
| `Escape` | メタデータパネル閉じる / 経路検索キャンセル |
| `P` | 経路検索モードのトグル |
| `[` / `]` | 左サイドバーの開閉 |

---

## トースト通知

グローバルな通知は画面右下に表示する。

| イベント | 表示 |
| --- | --- |
| ファイルアップロード完了 | ✅ "report.pdf をアップロードしました" |
| 処理完了 | ✅ "report.pdf の解析が完了しました — 開いて探索" |
| 処理失敗 | ❌ "report.pdf の処理が失敗しました — ログを確認" |
| 正規化承認待ち | 🛠 "report.pdf が承認待ちになりました" |
| 経路が見つからない | ⚠ "指定された経路は見つかりませんでした" |

---

## フロント実装メモ

- `GetGraph` の結果を `buildPaperMap` に渡して `paperMap` を生成する
- `summary_html` が null のノードは `description` を `Paper.content` に使う
- メタデータパネルを開いた時のみ `GetGraphEntityDetail` を呼ぶ
- `ExpandNeighbors` は初期スコープでは使用しない（paper-in-paper の展開モデルで代替）
- `FindPaths` は明示的な経路検索操作でのみ呼ぶ
- `GetDocument` または `ListDocuments` をポーリング（3 秒間隔）して処理ステータスをリアルタイム更新する。`completed` / `failed` になった時点でポーリングを停止する
- `PaperCanvas` の controlled API を使い、`paperMap` と `expansionMap` をアプリ側で管理する

---

## 開発者向け統計ビューワー（`/dev/stats`）

`/dev/stats` の各タブは metrics family に対応させる。

| Metrics Family | タブ | 主な API |
| --- | --- | --- |
| `PipelineMetrics` | パイプライン | `GetPipelineStats` |
| `ExtractionMetrics` | 抽出品質 | `GetExtractionStats` |
| `EvaluationMetrics` | 評価 | `GetEvaluationTrend` |
| `ErrorMetrics` | エラー | `ListFailedDocuments` |
| `NormalizationMetrics` | 正規化ツール | `ListNormalizationTools`, `GetNormalizationToolRun` |
| `AliasMetrics` | エイリアス管理 | `NodeService.ApproveAlias`, `NodeService.RejectAlias` |

### タブ 1: パイプライン統計 (`PipelineMetrics`)

```
┌─── ステージ別 処理時間 ────────────────────────────┐
│  semantic chunking   ████░░░░  avg 1.2s  p95 3.1s  │
│  pass 1 / chunk      ████████  avg 2.4s  p95 5.8s  │
│  pass 2              ██████░░  avg 3.1s  p95 7.2s  │
│  html summary        █████░░░  avg 1.8s  p95 4.3s  │
└────────────────────────────────────────────────────┘

┌─── 処理結果 ─────┐   ┌─── Gemini 呼び出し/コスト（日別）─┐
│ completed  94%   │   │  ▁▃▅▇▅▃▇▅  呼び出し数            │
│ failed      6%   │   │  ▁▂▃▄▃▂▄▃  コスト ($)            │
│ → 失敗一覧へ     │   └──────────────────────────────────┘
└──────────────────┘
```

**データソース**: `documents.status`, `processing_jobs`（ステージ別開始・完了時刻）, Gemini 呼び出しログ

### タブ 2: 抽出品質統計 (`ExtractionMetrics`)

```
┌─── ドキュメント選択 ──────────────────────────────┐
│  [全体 ▼]  または  [doc_001 ▼]                   │
└──────────────────────────────────────────────────┘

┌─── level 分布 ────────────┐  ┌─── category 分布 ─────────┐
│  level 0  ██░░░░░   3件   │  │  concept  ████████  58%   │
│  level 1  ████░░░  12件   │  │  entity   █████░░░  21%   │
│  level 2  ████████ 34件   │  │  claim    ███░░░░░  12%   │
│  level 3  ████████ 87件   │  │  evidence ██░░░░░░   7%   │
└───────────────────────────┘  │  counter  █░░░░░░░   2%   │
                                └───────────────────────────┘

┌─── Pass 1 → Pass 2 統合数 ──────────────────────┐
│  Pass 1 抽出: 136ノード                           │
│  Pass 2 後:   122ノード  (-14件 統合)             │
└──────────────────────────────────────────────────┘
```

### タブ 3: 評価トレンド (`EvaluationMetrics`)

```
┌─── Precision / Recall 週次推移 ─────────────────┐
│  1.0 │  ·─·─·  Precision                        │
│  0.6 │  ○─○─○  Recall                           │
│      └─── w1 ─── w2 ─── w3 ─── w4              │
└──────────────────────────────────────────────────┘
```

### タブ 4: エラー分析 (`ErrorMetrics`)

```
┌─── 失敗ドキュメント一覧 ─────────────────────────────────────┐
│  doc_023  JSON parse error  2026-03-27  [再処理]              │
│  doc_031  Gemini timeout    2026-03-26  [再処理]              │
└──────────────────────────────────────────────────────────────┘
```

- [再処理] は `StartProcessing(force_reprocess=true)` を呼ぶ

### タブ 5: 正規化ツール管理 (`NormalizationMetrics`)

`is_dev=true` のメンバーのみ。workspace 非依存のシステムグローバルなツール一覧。

```
┌─── 正規化ツール一覧 ──────────────────────────────────────────────┐
│  名前                         状態       問題パターン        操作  │
│  normalize_mojibake_shiftjis  approved   Shift-JIS文字化け  [実行] │
│  fix_csv_columns              reviewed   CSV列ずれ     [承認][実行]│
│  remove_pdf_noise             draft      PDFノイズ  [dry-run][削除]│
│                                                      [+ 新規生成]  │
└───────────────────────────────────────────────────────────────────┘
```

状態遷移ボタン:
- `draft` → [dry-run] のみ表示
- `reviewed` → [承認する] [再 dry-run] を表示
- `approved` → [本実行] [廃止にする] を表示

### タブ 6: エイリアス管理 (`AliasMetrics`)

`is_dev=true` のメンバーのみ。`node_aliases.merge_status=suggested` のエイリアス候補を一覧表示し、承認・却下を行う。

```
┌─── エイリアス候補 ───────────────────────────────────────────────┐
│  canonical          alias               score   操作              │
│  販売戦略 (cn_001)  Sales Strategy      0.92   [承認] [却下]      │
│  SNS施策 (cn_002)   ソーシャルメディア  0.89   [承認] [却下]      │
└──────────────────────────────────────────────────────────────────┘
```

- [承認] は `ApproveAlias` RPC を呼ぶ
- [却下] は `RejectAlias` RPC を呼ぶ

---

## Open Issues

- モバイル対応は初期スコープ外
- `summary_html` 内の `data-paper-id` リンク埋め込みを `html_summary_generation` ステージで生成する責務を [ai-pipeline.md](ai-pipeline.md) に追記する必要がある
- グラフのスナップショット共有（URL に展開状態を埋め込むか、サーバ側で保存するか）
