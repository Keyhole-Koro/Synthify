# クライアント item 負荷テスト

workspace の item 数が増えたときに、クライアント側で何がどれだけ重くなるかを再現・計測するための仕組み。

「item を多く持たせると重い」は 1 つの現象ではなく、**ペイロード / デコード / キャッシュ構築 / 再投影 / DOM・iframe / 保持メモリ** の重ね合わせになっている。1 本の E2E で測ると原因が混ざるので、レイヤを分けて測る。

## item 数 N で効いてくる箇所

| 箇所 | 何が起きるか |
|---|---|
| [`contracts/connectrpc/synthify/app/v1/tree_types.proto`](../../contracts/connectrpc/synthify/app/v1/tree_types.proto) `Item.content` | かつて `GetTree` が workspace の**全 item を content(HTML) 込みで一括返却**しており、ペイロードは N × content サイズだった。**現在は root node のぶんだけ返す**（下記「outline 化」）。この表の他の行は現在も有効 |
| [`apps/web/src/lib/connect.ts`](../../apps/web/src/lib/connect.ts) | `createConnectTransport` に `useBinaryFormat` を渡していないため connect-web は **JSON codec**。GetTree のデコードは `JSON.parse` + 全 item の `fromJson` になる（ただし実測では binary に変えても改善しない。下記参照） |
| [`workspaceTreeCache.ts`](../../apps/web/src/features/workspaces/tree/workspaceTreeCache.ts) `replaceWorkspaceTree` | 全 item を Map に展開し、item ごとに `SubtreeItem` message を確保してメモリ常駐 |
| [`useWorkspaceProjection.ts`](../../apps/web/src/features/workspaces/useWorkspaceProjection.ts) `projectWorkspacePapers` | **毎回全 item を再投影して新しい `Paper[]` を作る**。subtree 展開ごと・`treeChanged` refresh ごとに O(N) |
| [`useWorkspaceTree.ts`](../../apps/web/src/features/workspaces/useWorkspaceTree.ts) `loadSubtreeAndProject` | 展開 1 回 = subtree fetch + 全再投影 + `setWorkspacePapers` |
| paper-in-paper canvas | 開いている paper ごとに content iframe。DOM とメモリは item 数ではなく**開いている数**に比例する |
| （API 側）[`postgres/tree.go`](../../apps/api/internal/repository/postgres/tree.go) `populateChildIDs` | item ごとに `ListChildItems` を呼ぶ **1+N クエリ** |

## 3 レイヤ構成

| レイヤ | 何を測るか | 実行方法 | ブラウザ | バックエンド |
|---|---|---|---|---|
| **L0** データ層マイクロベンチ | decode / cache 構築 / 再投影 | `bun run bench` | 不要 | 不要 |
| **L1** ブラウザ実測 | React commit、long task、DOM/iframe、heap | `bun run e2e:perf` | 必要 | 不要（frontend のみ） |
| **L2** 実 API 込み | 実クエリ、実ペイロード、転送 | `scripts/seed_tree_items.sh` | 必要 | 必要 |

3 レイヤとも同じ形のツリーを食わせる。L0 と L1 は
[`mockTreeGenerator.ts`](../../apps/web/src/features/workspaces/tree/mockTreeGenerator.ts) を共有し、L2 の SQL は
同じ b-ary heap インデックス規則でツリーを組む。だから 3 つの数字を横に並べて比較できる。

## 負荷ノブ

`MockTreeSpec`（`mockTreeGenerator.ts`）と `InjectMockWorkspaceTreeArgs` が受け取る。

| ノブ | 効く先 |
|---|---|
| `totalItems` | cache サイズ、再投影コスト、GetTree の item 数 |
| `depth` | subtree ロードの連鎖、paper のネスト段数 |
| `branching` | 兄弟数（paper-in-paper の room 配分） |
| `contentBytes` | ペイロードのバイト数と保持メモリ。**item 数より支配的**なことが多い |
| `seed` | 決定性。再現できないベンチはベンチではない |

「どれだけ開いているか」は生成時のノブではなく、L1 側で **paper を実際にクリックして開く**ことで振る。
canvas の開閉状態は paper-in-paper が `OPEN_NODE` dispatch で持っており、canvas が mount した後に
アプリ側の `ExpansionMap` を書いても paper は開かないため（実測で確認済み）。

生成は `(workspaceId, spec)` に対して決定的。同じ id、同じタイトル、同じ content バイト数になる。

## L0: データ層マイクロベンチ

```sh
cd apps/web && bun run bench
```

[`treeLoad.bench.ts`](../../apps/web/src/features/workspaces/tree/treeLoad.bench.ts)。ブラウザなしで数十秒。回帰検知の主戦場。

### ベースライン (2026-07 / Linux container, content 2048B/item)

各行は mean。サイズは connect-web が運ぶ JSON の **UTF-8 バイト数**と、その gzip 後。
転送されるのは gzip 後だが、`JSON.parse` が舐めるのは展開後なので両方載せている。

| items | JSON | gzip | GetTree decode | replaceWorkspaceTree | projectWorkspacePapers |
|---|---|---|---|---|---|
| 100 | 0.28 MiB | 0.04 MiB | 2.1 ms | 0.06 ms | 0.011 ms |
| 1,000 | 2.80 MiB | 0.34 MiB | 21 ms | 1.1 ms | 0.28 ms |
| 5,000 | 14.01 MiB | 1.72 MiB | 110 ms | 4.8 ms | 1.0 ms |
| 20,000 | 56.08 MiB | 6.85 MiB | 535 ms | 30 ms | 17 ms |

参考: 5,000 item の workspace で paper を 1 つ開くコスト（subtree merge + 全再投影）は **1.6 ms**。

### 読み取れること

1. **支配的なのは decode で、再投影ではない。** 5,000 item で decode 110 ms に対し再投影 1.0 ms。約 110 倍の開き。
   「展開のたびに全 item を再投影している」のは事実だが、実測では当面のボトルネックではない。
2. **本当の問題はペイロード。** content を全 item 分積んで運ぶため、5,000 item で **14 MiB**（gzip 1.7 MiB）、
   20,000 item で **56 MiB**（gzip 6.9 MiB）。デコード時間はこの展開後バイト数にほぼ比例する。
   効くのは item 数そのものより `content` を全件返している設計。

### 候補となる改善の効果（実測）

bench の `candidate fixes` セクションが 5,000 item / content 2048B で直接比較する。

| 版 | JSON | gzip | binary | binary gzip | decode JSON | decode binary |
|---|---|---|---|---|---|---|
| 現状（content 全件） | 14.01 MiB | 1.72 MiB | 13.71 MiB | 1.74 MiB | 110 ms | 98 ms |
| content を遅延取得 | 0.78 MiB | 0.06 MiB | 0.53 MiB | 0.06 MiB | **14 ms** | 17 ms |

- **binary codec はほぼ効かない。** gzip 後はむしろ JSON より大きく（1.74 vs 1.72 MiB）、
  decode も JS ランタイム上では速くならない（`fromBinary` は byte 列から文字列を作り直すため）。
  1 行で切り替えられるが、この payload では投資対効果がない。
- **効くのは content を運ばないことだけ。** decode 110 ms → 14 ms（**7.5 倍**）、gzip 1.72 MiB → 0.06 MiB（**28 倍**）。

したがって改善の順序は:
   - a. `GetTree` から `content` を外す（一覧は title/description/child_ids まで、本文は開いた item だけ取る）
     → **実装済み**。下記「outline 化」を参照
   - b. `populateChildIDs` の 1+N を 1 クエリに畳む（L2 の節を参照）— 未実装
   - c. 再投影の差分化。5,000 item で 1.0 ms なので、(a)(b) の後でも優先度は低い
   - binary codec は上表のとおり効果がないので、やらない

### GetTree の outline 化（実装済み）

`GetTree` は root node の本文だけを返し、それ以外の node の本文は paper を開いたときに
`GetSubtree` で取る。

- クエリ: `ListItemOutlinesByWorkspace`（[db/queries/tree.sql](../../db/queries/tree.sql)）が
  非 root の `content` / `override_css` を空にして返す。DB から本文バイトを読む段階で落とすので、
  API のメモリと DB IO にも効く。PostgreSQL 16 実測で 5,000 node の本文 **11,868,890 バイト → 2,371 バイト**。
- repository: `GetTreeOutlineByWorkspace`。`GetTreeByWorkspace`（本文込み）は worker と `FindPaths` が
  引き続き使う。
- frontend: `replaceWorkspaceTree` は root だけを本文取得済みとして扱い、`shouldSkipSubtreeLoad` から
  「workspace の outline がある」条件を外した。`useExpansionWatcher` は子を持たない item でも
  subtree を取りに行く（leaf にも自分の本文があるため）。
- 本文が未取得の paper は `buildProjectedPaper` の既存フォールバックで description を表示し、
  `GetSubtree` が返り次第 本文に差し替わる。

共有リンクビューア（`ShareLinkTree`）は title / description しか描画していないので影響を受けない。

## L1: ブラウザ実測

```sh
cd apps/web && bun run e2e:perf
```

[`e2e/perf.spec.ts`](../../apps/web/e2e/perf.spec.ts) + [`e2e/helpers/perf.ts`](../../apps/web/e2e/helpers/perf.ts)。
`@perf` タグ付きで、通常の e2e suite からは `testIgnore` で外してある（遅く、かつ他プロセスと同居すると数字が濁るため）。

ツリーは `window.__synthifyDebug.createMockWorkspace()` でフロントエンド内に注入する。バックエンドを経由しないので、
API レイテンシや worker のスループットから**クライアント単体のコストだけを切り出せる**。

計測する指標:

- `injectMs` / `projectMs` — アプリ自身が返す値。`injectMs` は mock 生成 + cache 構築（実負荷では GetTree の decode + 同じ cache 構築に相当）、`projectMs` は paper 投影
- `renderMs` — 注入から root content iframe が可視になるまでの wall clock（React commit + style + layout + paint 込み）
- `reprojectMedianMs` — `reprojectWorkspace()` を 7 回回した中央値
- long task の件数 / 合計 ms / 最大 ms（PerformanceObserver、`buffered: true`）
- JS heap、DOM ノード数、`ScriptDuration` / `TaskDuration` / `LayoutCount`（CDP `Performance.getMetrics`）
- 開いている iframe 数

heap は読む前に CDP `HeapProfiler.collectGarbage` を打ってから取る。打たないと未回収分に支配されて実行ごとに数十 MB ぶれる。
`performance.measureUserAgentSpecificMemory()` は cross-origin isolation が要るので使っていない。

2 つの軸を振る:

- **scale** — paper を開かずに item 数を 100 → 5,000 まで
- **open-count** — item 数を 2,000 に固定して、cover report 内のリンクをクリックして開く paper 数を 0 → 6

後者が必要なのは、**item 数だけでは描画コストが決まらない**から。閉じた paper は安く、開いた paper は content iframe を 1 つ抱える。
開く操作は実際のクリック（`OPEN_NODE` dispatch）で行う。アプリ側の `ExpansionMap` を書くだけでは canvas 上の paper は開かない。

結果は `perf-samples.json` と `perf-table.md` として Playwright report に添付され、stdout にも表が出る。

### L1 ベースライン (2026-07 / Linux container, Chromium, 1280x720)

**item 数を振る（開いている paper は workspace の cover report のみ）**

| items | inject ms | project ms | render ms | reproject ms | long task 数 / 合計 ms | heap MB | DOM ノード |
|---|---|---|---|---|---|---|---|
| 100 | 10.6 | 0.8 | 279 | 0.10 | 0 / 0 | +3.3 | +416 |
| 500 | 42.4 | 1.8 | 309 | 0.40 | 0 / 0 | +5.8 | +516 |
| 2,000 | 85.3 | 3.3 | 349 | 1.20 | 1 / 90 | +14.5 | +604 |
| 5,000 | 168.4 | 6.5 | 422 | 3.10 | 2 / 244 | +26.7 | +263 |

**開く paper 数を振る（item 数は 2,000 固定）**

| 開いた paper | iframe 数 | content frame | long task 数 / 合計 ms | heap MB | DOM ノード |
|---|---|---|---|---|---|
| 0 | 1 | 0 | 2 / 141 | +12.3 | +348 |
| 1 | 2 | 1 | 3 / 188 | +13.6 | +663 |
| 3 | 4 | 3 | 3 / 198 | +14.6 | +969 |
| 6 | 7 | 6 | 5 / 344 | +16.7 | +1430 |

`render ms` は注入から cover report が可視になるまで。paper を開くクリック列は Playwright の
往復待ちが支配的なので `render ms` には含めず、long task / DOM / heap 側に出している。

### 読み取れること

1. **heap は item 数に比例、DOM は開いた paper 数に比例。** 5,000 item を持つだけで +27 MB だが DOM は増えない
   （閉じた paper は DOM を食わない）。逆に item 数を 2,000 に固定したまま paper を 6 枚開くと DOM は +348 → +1430、
   iframe は 1 → 7 に増える。**item 数だけでは描画コストは決まらない。**
2. **注入コストは item 数にほぼ線形**（100 → 5,000 で 10.6 ms → 168 ms）。これは mock 生成 + cache 構築で、
   実負荷では GetTree の decode + cache 構築に相当する。L0 の decode 実測（5,000 item で 110 ms）と桁が合う。
3. **再投影は 5,000 item でも 3.1 ms** で、L0 の 1.0 ms と同オーダー。やはりここは当面のボトルネックではない。
4. long task は 5,000 item 注入時に合計 244 ms（最大 178 ms）出る。1 フレームを大きく超えるので、
   この間 UI は止まる。減らすべきは注入そのものではなく、その入力サイズ（= ペイロード）。

### 閾値を置いていない理由

L1 には wall-clock の閾値アサーションを入れていない。共有 CI ハードウェアでは flaky 製造機になるだけで、
得られるのは「遅い」という既知の情報だけになる。**数字そのものが成果物**で、回帰ゲートは L0 側に置く。
L1 のアサーションは「その scale が完走して item 数ぶんの paper ができたか」「開いた枚数ぶん content frame が増えたか」
だけ — 後者は、paper が実は開いていないのに軸を測ったつもりになる事故を防ぐためのもの。実際に一度その事故を起こしている。

## L2: 実 API 込み

```sh
scripts/seed_tree_items.sh <workspace_id> [total_items] [branching] [depth] [content_bytes]
# 例: scripts/seed_tree_items.sh ws_seed_1 5000 6 5 2048
```

[`scripts/seed_tree_items.sh`](../../scripts/seed_tree_items.sh) が compose の CockroachDB に直接 `tree_items` を投入する。
再実行すると自前で入れた行だけを消して入れ直す（id が `loadtest_` 前置きのものだけを削除するので、実 job が作ったノードには触らない）。

seed 後に workspace を開き、DevTools の Network で `GetTree` の転送量と時間を見る。L0 の decode 時間と突き合わせれば、
壁時計のうちどれだけが転送で、どれだけがクライアント処理かが分離できる。

ここで初めて `populateChildIDs` の 1+N クエリが表に出る。

### 実測 (PostgreSQL 16, 5,000 items x 2 KiB)

seed した実データに対して `GetTreeByWorkspace` が実際に投げる 2 本のクエリを流したもの。

| 処理 | 時間 |
|---|---|
| `ListItemsByWorkspace`（1 クエリ） | 7.4 ms |
| `populateChildIDs`（`ListChildItems` を 5,000 回） | 43.3 ms |

さらに `ListChildItems` は `content` を含む全カラムを SELECT する（[db/queries/tree.sql:141](../../db/queries/tree.sql)）ため、
**同じ 11.32 MiB の content を 2 回読んでいる**。child_ids を組み立てるだけなら id と parent_id で足りる。

この 43.3 ms はサーバ内ループでの計測なので、Go 側が 5,000 回ぶん払う round-trip は含まれていない。実際はさらに悪い。

改善は単純で、`populateChildIDs` を 1 クエリに畳める:

```sql
SELECT parent_id, array_agg(id) FROM tree_items
WHERE workspace_id = $1 AND parent_id IS NOT NULL GROUP BY parent_id;
```

### 検証状況

seed SQL は **PostgreSQL 16 に対して実行済み**（5,000 件ちょうど、level 分布 1/6/36/216/1296/3445、
orphan 0、content 11.87 MB、再実行で件数が二重にならないことを確認）。
migration は Postgres 方言で書かれており CockroachDB もこれを受けるが、
**`docker compose exec ... cockroach sql` 経由での実行はまだ通していない**（Docker Hub の blob CDN が
このセッションの egress policy で拒否されたため、CockroachDB イメージを取得できなかった）。
script のラッパ部分（compose exec、`--format=csv` の件数パース）も同じ理由で未実行。

`depth` が浅すぎると `total_items` に届かないまま打ち切られる（実測: `total=5000 depth=3 branching=6` で 259 件）。
実行時は件数出力 `seeded_items` が指定値と一致するか確認すること。

## 使い方（デバッグコンソール）

```js
// 2000 item、深さ 4、分岐 6、1 item あたり 2 KiB で注入
__synthifyDebug.createMockWorkspace({ totalItems: 2000, depth: 4, branching: 6, contentBytes: 2048 })

// 全再投影のコストを 10 回計測（中央値・最小・最大が返る）
__synthifyDebug.reprojectWorkspace('debug_ws_xxxx', 10)
```

`__synthifyDebug` は `NODE_ENV !== 'production'` のときだけ生える。

従来の `documentCount` / `nodesPerDocument` も引き続き使える（新しい形状ノブを併用しない限り、以前と同じフラットな形になる）。

## 今後

- L0 をベースライン比の回帰ゲートとして CI に入れる（まずは数回分の実測ばらつきを見てから閾値を決める）
- 上の改善順序 a → b を実施し、bench の `candidate fixes` セクションで前後比較する
- `populateChildIDs` を 1 クエリに畳む（上記 SQL）
- seed script を CockroachDB 実機で 1 度通す
- L2 を nightly に載せるかは、compose 込みの実行時間を見てから判断する
