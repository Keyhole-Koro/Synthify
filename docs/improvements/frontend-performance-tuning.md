# Frontend Performance Tuning Plan

## 現状の課題分析 (2026-06-09)

フロントエンド（`apps/web`）の表示速度および操作感において、以下のボトルネックが推測されます。

### 1. 初期化プロセスのウォーターフォール (Initialization Waterfall)
Next.js App Router と Firebase Auth、および `useWorkspaceTree` の初期化が直列（シーケンシャル）に発生しています。
- Firebase Auth の解決を待ってから `listWorkspaces` を実行。
- データの取得完了後に `PaperCanvas` (dynamic import) のロードを開始。
- Static Export (`output: 'export'`) のため、SSR による先行読み込みが効かず、クライアントサイドでの解決に依存している。

### 2. 巨大なベンダーライブラリのロード
知識ツリーの描画を担う `@keyhole-koro/paper-in-paper` が `dynamic` import されており、メインバンドルのロード完了後に再度フェッチが発生します。このライブラリ自体の初期化コストも無視できない可能性があります。

### 3. ツリー投影 (Tree Projection) の計算コスト
`useWorkspaceTree.ts` において、Firestore から取得したデータを `Paper` 形式に変換する `projectWorkspacePapers` がレンダリングごとに実行される可能性があります。ノード数が増大した場合、メインスレッドを占有し UI のカクつき（Jank）の原因となります。

---

## 推奨されるプロファイリングツール

パフォーマンスの問題を定量的・定性的に特定するために以下のツールを活用します。

| ツール | 用途 | 確認項目 |
| :--- | :--- | :--- |
| **Next.js Bundle Analyzer** | バンドルサイズの可視化 | `paper-in-paper` や `framer-motion` の占有率 |
| **React Profiler** | コンポーネントの再レンダリング計測 | 不要なレンダリングや `useMemo` の不足箇所 |
| **Chrome DevTools (Performance)** | メインスレッドの解析 | Long Task (50ms+) の特定、LCP の詳細 |
| **New Relic Browser** | 実環境でのパフォーマンス収集 | `soft_navigations` や AJAX の遅延傾向 |

---

## 具体的な改善アプローチ

### 1. 投機的実行と Session Hint の活用
Firebase Auth の解決を待たずに、`localStorage` に保存された「前回ログイン済みフラグ (Session Hint)」を元に、API リクエストを早期に開始します。
- **メリット**: Auth SDK の初期化待ち時間（数百ms〜数秒）を API 通信時間とオーバーラップさせることができます。
- **留意点**: トークンが未発行の状態でリクエストが飛ばないよう、API クライアント側での制御が必要です。

### 2. API クライアントでのトークン取得待ちの隠蔽
API 通信時にトークンがまだ取得できていない場合、呼び出し元にエラーを返すのではなく、API クライアント内部で「トークンが利用可能になるまで待機するキュー」を実装します。
- **メリット**: UI コンポーネント側は Auth の状態を意識せずに `fetch` を開始でき、コードの複雑性を抑えつつ並列化を実現できます。

### 3. スケルトン UI (Skeleton Screen) の導入
データ取得中に真っ白な画面やスピナーを見せるのではなく、ツリー構造やワークスペース一覧の「型」を先に表示します。
- **メリット**: LCP (Largest Contentful Paint) の改善とともに、ユーザーに「アプリが動いている」という安心感を与え、体感速度を向上させます。
- **実装**: `PaperCanvas` のロード中も、背景グラデーションやヘッダーなどの静的要素を即座に表示し、レイアウトシフトを最小限に抑えます。

---

## 計測で判明した事実 (2026-06-11)

`next experimental-analyze` でトップルート (`app/page.tsx`) のバンドルを解析した結果、
**最大のボトルネックは Firestore がトップページの初期ロードに同梱されていること**と確定した。

### F-1. Firestore (74.66KB gzip) がトップの初期チャンクに乗っている【最優先】
共有チャンク `common-*.esm.js`（74.66KB gzip / 252KB raw）の中身は **Firestore 本体**。
アナライザの Import Chain が以下のように繋がっており、トップページが静的 import で
Firestore 全体を初期ロードに引き込んでいる:

```
app/page.tsx
  └─ src/features/landing/useLandingPageController.ts  (line 6)
       └─ import { collection, getDocs, limit, orderBy, query } from 'firebase/firestore'
            └─ @firebase/firestore/index.esm.js
                 └─ common-*.esm.js  (74.66KB gzip)
```

- **原因**: [useLandingPageController.ts:6](../../apps/web/src/features/landing/useLandingPageController.ts) の
  `firebase/firestore` トップレベル静的 import。`db` も `@/lib/firebase` から静的 import。
- **打ち手**: Firestore を**動的 import 化**（実データ取得時に `await import('firebase/firestore')`）。
  Firestore はログイン後のワークスペース取得時に初めて要るので、初期表示には不要。
- **期待効果**: 初期 JS から **~75KB(gzip)** 削減。課題1の初期化ウォーターフォール短縮にも寄与。
- **留意点**: `db` 初期化・`useWorkspaceTree` など複数箇所が `firebase/firestore` に依存するため、
  動的化の前に依存箇所の洗い出しが必要。

### F-2. framer-motion を最小用途のために全量ロード【次点】
- `framer-motion` はアプリ本体 (`src/`) では未使用。**vender の `paper-in-paper` の
  `PaperNodeFrame.tsx` 1ファイルのみ**が使用。
- 使用 API は `motion.div`（initial/animate/exit で `x/y/opacity/scale`）＋ `AnimatePresence` のみ。
  drag・layout・variants・gesture 等の高度な機能は**未使用**（アナライザに見える drag/layout は
  ライブラリの export であって使用箇所ではない）。
- **打ち手**: paper-in-paper 側で CSS transition + 軽量な presence 制御に置換すれば
  framer-motion を丸ごと外せる（**~30-40KB gzip** 級）。ただし vender ライブラリの改修。

### F-3. paper-in-paper は遅延分離できている【現状維持】
- アナライザのトップルート初期チャンクに `@keyhole-koro/paper-in-paper` は出現しない
  ＝ `dynamic()` による遅延分割が効いている。現状で問題なし。

---

## 改善ロードマップ (提案)

### Phase 1: 可視化と計測
- [ ] `@next/bundle-analyzer` の導入。
- [ ] New Relic Browser の `distributed_tracing` および `performance` 設定の有効化。
- [ ] クリティカルパス上の API リクエストの並列化（`Promise.all` の活用強化）。

### Phase 2: バンドルサイズの最適化
- [ ] `paper-in-paper` の prefetch 検討。
- [ ] 未使用ライブラリの削減、および Tree Shaking の確認。

### Phase 3: レンダリングパフォーマンスの向上
- [ ] `useWorkspaceTree` 内の計算処理への `useMemo` 徹底。
- [ ] `canvas` 描画負荷が高い場合の React コンポーネント境界の最適化。

---

## 付録: ツール導入の具体手順

### バンドル解析 (`next experimental-analyze`)
> **注意**: `@next/bundle-analyzer` は本プロジェクトの **Turbopack ビルドと非互換**で、
> レポートを一切生成しません（実際に導入して確認済み, 2026-06-11）。
> Next.js 16 ネイティブの `next experimental-analyze` を使います。

`apps/web/package.json` に以下のスクリプトを定義済み:
```json
"analyze": "next experimental-analyze"
```

実行:
```bash
cd apps/web && bun run analyze
# → http://localhost:4000 でインタラクティブ UI が起動
```

主なオプション:
- `-o, --output` … UI を起動せず解析ファイルだけ出力。
- `--profile` … React の本番プロファイリングを有効化。
- `--port <port>` … ポート変更（既定 4000）。

UI の読み方は「## バンドル解析の読み方」を参照。

---

## バンドル解析の読み方

数字を見る順番は **①初期ロードに乗る JS → ②内訳で誰が太いか → ③遅延できないか**。
合計サイズより「クリティカルパス（初期チャンク）に乗っているか」を優先して見る。

- **初期チャンクかどうかの判定**: アナライザの各チャンクの **Import Chain** を見る。
  チェーンが `app/page.tsx` 等のページから静的 import で辿れる場合、そのチャンクは初期ロードに乗る。
  （Network タブはチャンク名がハッシュで読みづらいので、Import Chain で判定する方が確実。）
- **ツリーマップの面積 = バイトサイズ**。最大の矩形から潰す。
- **同名モジュールの複数出現 = 重複バンドル**。dedupe で削れる。本プロジェクトは
  `vender/paper-in-paper` を `file:` 依存にしているため react / framer-motion / firebase の
  二重持ちが起きやすい。要注意。
- **gzip/brotli 後のサイズで判断**（raw だけ見ない）。
- 目安: Static Export の SPA では **First Load JS が ~200KB(gzip) を超えると初期表示が体感で重い**。

このプロジェクトで名指しで疑う対象: `firebase`（特に firestore / webchannel-wrapper）、
`@keyhole-koro/paper-in-paper`、`framer-motion`。

---

## Before / After 計測手順

改善の効果を定量化するため、変更の前後で同じ手順を踏んで比較する。
**毎回同じ条件**（同一マシン・CPU/Network throttle あり・キャッシュ無効）で測ること。

### 計測する指標
| 指標 | 取得方法 | 意味 |
| :--- | :--- | :--- |
| **対象チャンクの gzip サイズ** | `next experimental-analyze` | 例: `common-*.esm.js`（Firestore）の縮減量 |
| **First Load JS** | `bun run build` のビルドサマリ or アナライザ | トップルートの初期 JS 合計 |
| **LCP / FCP** | Chrome DevTools → Performance | 体感の初期表示速度 |
| **Long Task (50ms+)** | Chrome DevTools → Performance → Main | メインスレッド占有 |
| **初期化ウォーターフォール** | Chrome DevTools → Network | auth→firestore→canvas の直列段数 |

### 手順

#### 0. 計測条件を固定
- Chrome DevTools → Performance/Network → CPU **4x slowdown**、Network **Slow 4G**、**Disable cache** にチェック。
- 計測対象ページは `/`（トップ）。ログイン状態も before/after で揃える。

#### 1. Before を記録（変更前）
```bash
cd apps/web
git stash            # 作業中変更があれば退避（クリーンな現状で測る）
bun run analyze -o   # 解析ファイルを出力（UI 起動なしで CI 的に取得する場合）
# または UI で対象チャンクの gzip サイズを目視記録
```
- アナライザで **`common-*.esm.js`（Firestore）の gzip サイズ**を記録 → 現状 **74.66KB**。
- `bun run build` のサマリで `/` の **First Load JS** を記録。
- DevTools Performance で `/` をリロード録画し、**LCP / FCP / Long Task / Network の段数**を記録。
- 数値は下表 (## 計測ログ) に追記。

#### 2. 変更を適用
- 例: F-1（Firestore 動的 import 化）を実装。

#### 3. After を記録（変更後）
- 1 と**完全に同じ手順**で再計測。
- 期待: `common-*.esm.js`（Firestore）がトップの初期チャンクから消える、
  または別の遅延チャンクに移る。First Load JS が ~75KB 減。

#### 4. 比較してログに残す
- 差分を ## 計測ログ に記録。回帰（増えた）場合は原因を Import Chain で追う。

### 計測ログ
> First Load JS = `out/index.html` が参照する初期ロード JS チャンクの gzip 合計（下記コマンドで実測）。
> Next 16 / Turbopack のビルドサマリには数値列が出ないため、`out/` から直接集計する。

ベースライン実測 (2026-06-11, `bun run build` 後の `out/`):

| 日付 | 変更 | Firestore chunk (gzip) | First Load JS (gzip) | LCP | 備考 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 2026-06-11 | （Before / 変更なし） | 72.68KB (`0_o3p_4s.wxa8.js`) | **513.40KB** | TBD | 初期ロードが目安200KBの2.5倍。Firestore 同梱 |
| | F-1 適用後 | | | | |

- **First Load JS 513.40KB(gzip)** は目安 ~200KB の 2.5 倍超で、初期ロードが重い状態。
- 上位チャンク gzip: `0-ayq864m5vmo`=136KB / **`0_o3p_4s.wxa8`=72.68KB(≒Firestore)** /
  `0-6sco1aqa7y8`=65.9KB / `0w-mxglfwz5.9`=60.9KB。
- **LCP は未取得（要 DevTools Performance 録画, throttle あり）**。
- 各チャンクの中身同定（136KB の最大チャンクに何が入っているか）は未実施。要追調査。

#### First Load JS の実測コマンド
```bash
cd apps/web
set -a && . ../../.env && set +a   # ルート .env の NEXT_PUBLIC_FIREBASE_* を読み込む
bun run build
# index.html が参照する初期チャンクの gzip 合計を出す:
files=$(grep -oE '/_next/static/chunks/[^"]+\.js' out/index.html | sort -u | sed 's#^/#out/#')
total=0; for f in $files; do [ -f "$f" ] && total=$((total+$(gzip -c "$f" | wc -c))); done
echo "First Load JS (gzip): $((total/1024)) KB"
```
> 注: ビルドにはルートの `.env`（`NEXT_PUBLIC_FIREBASE_*`）が必要。
> `apps/web` には `.env` が無く、未読込だと env バリデーションでビルドが失敗する。

