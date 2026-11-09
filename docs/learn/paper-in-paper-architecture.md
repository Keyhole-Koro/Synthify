# Paper-in-Paper 状態管理アーキテクチャ

`paper-in-paper`（`apps/web/vender/paper-in-paper`、0.4.0）の状態管理と描画の仕組み、そして Synthify からの使い方をまとめます。ライブラリ側の詳細は `apps/web/vender/paper-in-paper/docs/` を参照してください。

## 1. 外部ストア

- `PaperCanvas` は 1 つのストア（`createPaperStore`）を作り、React の state ではなくこのストアに状態を持ちます。
- `dispatch(command)` で純粋な reducer を通したあと、空間管理（窮屈な部屋のペーパーをインデックス化・クローズ）を収束まで適用し、レイアウトを計算してから、購読者に 1 回だけ通知します。
- スナップショット `{ state, config, rootId, layout, indexLabels }` は変更があったときだけ差し替わります。
- **関連ファイル**: `src/lib/react/store/createPaperStore.ts`, `src/lib/core/spaceManagement.ts`

## 2. ペーパー単位の購読

- 各 `PaperNode`（memo）が購読するのは「自分のレイアウトエントリ」「自分の `Paper`」「自分がフォーカスされているか」の 3 つだけです（`useSyncExternalStore`）。
- レイアウトのエントリは値が変わらなければ同じオブジェクトが使い回されます。注意度は時刻ではなく「操作のクロック」で減衰するので、何もしなければレイアウトは 1px も変わりません。
- その結果、ホストから 1 ペーパーだけ更新されたときに再描画されるのはそのペーパーだけ、ドラッグ中のポインタ移動では再描画されるペーパーはありません（`test/react/renderCount.test.tsx` で固定）。
- **関連ファイル**: `src/lib/react/components/PaperNode.tsx`, `src/lib/react/store/storeContext.ts`

## 3. ドラッグは別ストア

- ドラッグのセッション・ドロップ先・ポインタ位置は別のストアに持ちます。ポインタ位置を購読するのはゴーストだけ、ドロップ先の変化で再描画されるのは関係する部屋だけです。
- 自分のサブツリーはドロップ先になりません（木が循環しないように）。

## 4. 壊れない入力

- reducer は無効なコマンド（存在しない id、子ではないノードを開く、自分のサブツリーへの移動など）では state を変えません。
- 本文中の `data-paper-id` リンクは、子なら開き、それ以外の既知のペーパー（親・兄弟・別の枝）はツリー上の本来の場所で表示します。LLM が親へ戻るリンクを書いても壊れません（以前はキャンバスごとクラッシュしていました）。
- レイアウトは木と整合する展開エントリだけをたどります。localStorage から復元した展開状態に古いエントリや循環があっても描画は壊れません。
- HTML 本文の iframe には CSP（ドキュメントごとの nonce）が付き、本文中のスクリプトやインラインハンドラは実行されません。外部のスタイルシートやフォントも読み込めません。
- iframe 内の bootstrap は本文より先に `<head>` で動き、DOM の関数を先に確保するので、本文の DOM clobbering（`<img name="addEventListener">` など）で壊れません。キャンバスとの通信はドキュメントごとの秘密値で開いた専用の `MessageChannel` だけで行い、window のメッセージは受け付けません。
- iframe が自分のドキュメント以外を表示したら（meta refresh などで遷移した場合）、そのペーパーの本文は `labels.contentBlocked` の通知に置き換わります。
- 子ペーパーの DOM 順は `childIds` 順で固定です。並べ替えても iframe は DOM 上で動かないので再読み込みされません（レイアウト順は `data-child-index`）。

## 5. Synthify 側の使い方

| 箇所 | 内容 |
|---|---|
| `features/landing/LandingPageView.tsx` | `PaperCanvas` を描画。`defaultOpenState` には未読み込みのペーパーを含めてよい（読み込まれた時点で開く）。日本語ラベル（`CANVAS_LABELS`）、並べ替えのみ許可する `canDrop`（サーバーに移動 API がないため） |
| `features/paperMap/hooks/usePersistentPaperOpenState.ts` | `onExpansionMapChange` / `onFocusedNodeIdChange` を localStorage に保存（コールバックは変化時のみ、マウント時は呼ばれない）。ユーザーが替わったら、復元した状態とキャンバスの `key` を同じ更新で切り替える。保存できるのは最新の解決に対応するキャンバスだけで、入れ替わり中の古いキャンバスの通知は捨てる（前のユーザーの状態が次のユーザーに保存されない） |
| `features/paperMap/expansionPersistence.ts` | 復元時に不正なエントリを捨て、件数に上限を設ける |
| `features/workspaces/paper/components/WorkspaceRootContent.tsx` | カバーレポートを `PaperHtmlContent` で描画（独自の iframe ブリッジは廃止） |
| `features/workspaces/useWorkspaceProjection.ts` | ツリー → `Paper` の変換。値が同じアイテムは前回の `Paper` オブジェクトを使い回す（変わったペーパーだけ再描画させるため） |

`vite.config.ts` / `vitest.config.ts` はライブラリをソースのまま読み込み、`resolve.dedupe` で React をアプリ側の 1 つに揃えています（サブモジュール内に `node_modules` がある開発時でも React が二重にならないように）。
