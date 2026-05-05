# Workspace Paper Compact UI

workspace paper の役割を「常時大きい管理パネル」から「tree の起点 + 必要時だけ管理 UI を見せる compact handle」に再定義するための UI 仕様メモ。

## 背景

現状の workspace paper は以下を同時に担おうとしている。

- upload 入口
- current job status
- recent jobs
- document root 一覧
- tree の起点

この形だと、tree が生成された後も workspace paper が広い面積を占有し続け、paper-in-paper 上で本来主役である document tree の表示領域を圧迫する。

また、workspace 内の document roots を workspace paper 内の一覧としても、右側の papers としても見せると二重表現になりやすい。

## 目標

- tree がまだ無い workspace では、今の upload-centered UI を維持する
- tree がある workspace では、workspace paper を compact にする
- document roots は workspace paper の中の一覧ではなく、paper-in-paper の child papers として直接見せる
- upload / status / jobs は hover または focus 時に詳細表示する
- internal workspace root item は UI に出さない

## UI モード

workspace paper は `empty` と `populated` の 2 モードを持つ。

### 1. Empty Workspace

対象:

- document root が 0 件
- まだ tree が存在しない workspace

見え方:

```text
+--------------------------------------+
| Workspace                            |
| Act Team                             |
|                                      |
|      [ upload icon ]                 |
|      ファイルをアップロード           |
|   クリックまたはドラッグ&ドロップ     |
|                                      |
|      解析結果はここから始まる         |
+--------------------------------------+
```

ルール:

- 今の大きい upload/dropzone UI を維持する
- onboarding 面として振る舞う
- compact 表示にはしない

### 2. Populated Workspace

対象:

- document root が 1 件以上
- tree が存在する workspace

通常時の見え方:

```text
+-----------------------------+
| Act Team            +upload |
| 3 docs        running 72%   |
+-----------------------------+
          |
          +- [Act Software Documentation]
          +- [API Notes]
          \- [Meeting Transcript]
```

ルール:

- workspace paper 自体は compact handle として表示する
- 常時表示は最小限に絞る
  - workspace 名
  - document 数
  - upload の小ボタン
  - running 中だけ進捗の要約
- document roots は workspace paper の child papers として右側に直接出す
- workspace paper 内に `Documents` の重複一覧は持たない

### 3. Populated Workspace Hover / Focus

見え方:

```text
+--------------------------------------+
| Act Team                             |
| 3 docs                        +upload |
|                                      |
| Current Job                          |
| chunking...                    72%   |
| [██████████----]                     |
|                                      |
| Recent Jobs                          |
| succeeded / failed / running         |
+--------------------------------------+
          |
          +- [Act Software Documentation]
          +- [API Notes]
          \- [Meeting Transcript]
```

ルール:

- hover または focus 中だけ詳細管理 UI を展開する
- 展開内容:
  - upload action
  - current job status
  - progress bar
  - recent jobs
- モバイルは考慮対象外なので hover 前提を許容する
- ただし hover だけでなく focused/selected 状態でも開ける方が望ましい

## Completed After Upload

`completed` 後の契約は次のとおり。

- tree を refresh する
- internal workspace root item は解決に使うだけで UI には出さない
- 新しい document root を workspace の child papers に追加する
- 既存 branch は壊さない
- 視点移動や自動ズームはしない
- running summary は消し、必要なら短い success state を一時表示する

例:

```text
+-----------------------------+
| Act Team                    |
| 4 docs              +upload |
| just completed              |
+-----------------------------+
```

## Tree Projection Rule

DB / backend 上には workspace root item が存在しても、frontend では表示しない。

例:

- internal root item: `test`
- visible document root: `Act Software Documentation`

frontend の投影ルール:

1. workspace root item を内部的に解決する
2. その子である document roots を `workspace` の直接の child papers として扱う
3. document root 以下の subtree は通常どおり paper-in-paper で展開する

つまり user が見る構造はこうなる。

```text
workspace
|
+- document root A
|  |
|  +- section A-1
|  \- section A-2
+- document root B
\- document root C
```

user にはこうは見せない。

```text
workspace
|
\- internal root item
   |
   +- document root A
   +- document root B
   \- document root C
```

## State Summary

### Empty

- full upload UI
- document tree はまだ無い

### Populated Default

- compact workspace handle
- document roots は外側の child papers
- current job が無ければ静かな表示

### Populated Running

- compact handle の中に small progress summary を出す
- 詳細は hover/focus overlay

### Populated Completed

- new document root を child papers に追加
- success を短く示す
- 強い画面遷移はしない

## 実装メモ

frontend 側では以下の責務分離が必要。

- `WorkspacePaper`
  - empty / compact / expanded の表示切り替え
  - upload と job status の表示制御
- `useWorkspaceTree`
  - internal workspace root item を隠す投影
  - document roots を workspace child papers として再構成
  - completed 後の refresh と branch 追加
- `LandingPage`
  - `PaperCanvas` と循環同期しない controlled な `paperMap` / `expansionMap` 管理

## 非目標

- モバイル UI 最適化
- backend schema の変更
- workspace root item 自体の保存構造変更
- paper-in-paper 汎用仕様の変更
