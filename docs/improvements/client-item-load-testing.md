# クライアント item 負荷テスト

workspace の item 数が増えたときに、クライアント側で何がどれだけ重くなるかを再現・計測するための仕組み。

「item を多く持たせると重い」は 1 つの現象ではなく、**ペイロード / デコード / キャッシュ構築 / 再投影 / DOM・iframe / 保持メモリ** の重ね合わせになっている。1 本の E2E で測ると原因が混ざるので、レイヤを分けて測る。

## item 数 N で効いてくる箇所

| 箇所 | 何が起きるか |
|---|---|
| [`contracts/connectrpc/synthify/app/v1/tree_types.proto`](../../contracts/connectrpc/synthify/app/v1/tree_types.proto) `Item.content` | `GetTree` は workspace の**全 item を content(HTML) 込みで一括返却**する。ペイロードは N × content サイズ |
| [`apps/web/src/lib/connect.ts`](../../apps/web/src/lib/connect.ts) | `createConnectTransport` に `useBinaryFormat` を渡していないため connect-web は **JSON codec**。GetTree のデコードは `JSON.parse` + 全 item の `fromJson` になる |
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
| `openDepth` | 実際に開く階層数 = DOM ノード数と iframe 数 |
| `seed` | 決定性。再現できないベンチはベンチではない |

生成は `(workspaceId, spec)` に対して決定的。同じ id、同じタイトル、同じ content バイト数になる。

## L0: データ層マイクロベンチ

```sh
cd apps/web && bun run bench
```

[`treeLoad.bench.ts`](../../apps/web/src/features/workspaces/tree/treeLoad.bench.ts)。ブラウザなしで数十秒。回帰検知の主戦場。

### ベースライン (2026-07 / Linux container, content 2048B/item)

各行は mean。`wire` は connect-web が実際に運ぶ JSON のバイト数。

| items | wire (JSON) | GetTree decode | replaceWorkspaceTree | projectWorkspacePapers |
|---|---|---|---|---|
| 100 | 0.22 MiB | 2.3 ms | 0.13 ms | 0.017 ms |
| 1,000 | 2.18 MiB | 23 ms | 0.87 ms | 0.23 ms |
| 5,000 | 10.93 MiB | 114 ms | 7.6 ms | 1.3 ms |
| 20,000 | 43.77 MiB | 469 ms | 56 ms | 30 ms |

参考: 5,000 item の workspace で paper を 1 つ開くコスト（subtree merge + 全再投影）は **1.3 ms**。

### 読み取れること

1. **支配的なのは decode で、再投影ではない。** 5,000 item で decode 114 ms に対し再投影 1.3 ms。約 90 倍の開き。
   「展開のたびに全 item を再投影している」のは事実だが、実測では当面のボトルネックではない。
2. **本当の問題はペイロード。** content を全 item 分積んで JSON で運ぶため、5,000 item で **約 11 MiB**、
   20,000 item で **約 44 MiB**。デコード時間はこのバイト数にほぼ比例する。効くのは item 数そのものより
   `content` を全件返している設計。
3. **改善の効き順**はこの数字から決まる。
   - a. `GetTree` から `content` を外す（一覧は title/description/child_ids まで、本文は開いた item だけ `GetSubtree` / `GetTreeEntityDetail` で取る）
   - b. connect-web を binary codec にする（`useBinaryFormat: true`）
   - c. 再投影の差分化（(a)(b) の後で初めて意味を持つ）

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

- **scale** — `openDepth` を 1 に固定して item 数を 100 → 5,000 まで
- **open-depth** — item 数を 2,000 に固定して `openDepth` を 1 → 3

後者が必要なのは、**item 数だけでは描画コストが決まらない**から。閉じた paper は安く、開いた paper は content iframe を 1 つ抱える。

結果は `perf-samples.json` と `perf-table.md` として Playwright report に添付され、stdout にも表が出る。

### L1 ベースライン

**未取得。** L1 は compose（frontend + Firebase Auth emulator）が要るため、まだ実機で 1 度も通していない。
harness 側（`__synthifyDebug` 経由の注入、`reprojectWorkspace`、CDP の heap / `ScriptDuration` 取得、long task observer）は
`next dev` 単体に対して動作確認済みだが、**paper が実際に描画された状態での数字はこれから取る**。
最初に `bun run e2e:perf` を通した人がこの節に表を貼ること。

### 閾値を置いていない理由

L1 には wall-clock の閾値アサーションを入れていない。共有 CI ハードウェアでは flaky 製造機になるだけで、
得られるのは「遅い」という既知の情報だけになる。**数字そのものが成果物**で、回帰ゲートは L0 側に置く。
L1 のアサーションは「その scale が完走して item 数ぶんの paper ができたか」「`openDepth` を上げたら実際に DOM が増えたか」
だけ — 後者は、ツリーが実は展開されていないのに軸を測ったつもりになる事故を防ぐためのもの。

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

**未実行。** この SQL は CockroachDB に対してまだ流していない（作成環境に docker daemon がなかった）。
初回実行時は件数出力（`seeded_items`）が指定した `total_items` と一致するかを必ず確認すること。
`depth` が浅すぎると `total_items` に届かないまま打ち切られる。

## 使い方（デバッグコンソール）

```js
// 2000 item、深さ 4、分岐 6、1 item あたり 2 KiB、2 階層開いた状態で注入
__synthifyDebug.createMockWorkspace({ totalItems: 2000, depth: 4, branching: 6, contentBytes: 2048, openDepth: 2 })

// 全再投影のコストを 10 回計測（中央値・最小・最大が返る）
__synthifyDebug.reprojectWorkspace('debug_ws_xxxx', 10)
```

`__synthifyDebug` は `NODE_ENV !== 'production'` のときだけ生える。

従来の `documentCount` / `nodesPerDocument` も引き続き使える（新しい形状ノブを併用しない限り、以前と同じフラットな形になる）。

## 今後

- L0 をベースライン比の回帰ゲートとして CI に入れる（まずは数回分の実測ばらつきを見てから閾値を決める）
- 上の「改善の効き順」a → b を実施し、同じベンチで前後比較する
- L2 を nightly に載せるかは、compose 込みの実行時間を見てから判断する
