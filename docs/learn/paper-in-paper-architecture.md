# Paper-in-Paper 状態管理アーキテクチャ

このドキュメントでは、`paper-in-paper` ライブラリにおける状態管理とパフォーマンス最適化の仕組みについてまとめています。

## 1. 購読型 Context パターン (Pub/Sub in Context)

- **概要**: `PaperStoreProvider` が `state` そのものではなく、`api`（`subscribe`, `getSnapshot`）を Context で配信する仕組み。
- **仕組み**:
    - `PaperStoreProvider` 内で `useRef` を使用してストアの実体（`listeners`, `snapshot`, `api`）を保持。
    - Context には固定された `api` オブジェクトを流す。
- **メリット**: 通常の Context は値が変わると配下のコンポーネントがすべて再描画されますが、この方式は「通知」だけを飛ばすため、無関係なコンポーネントを再描画から守ります。
- **関連ファイル**: `apps/web/vender/paper-in-paper/src/lib/react/context/PaperStoreContext.tsx`

## 2. セレクター関数 (Selector Pattern)

- **概要**: `usePaperStoreSelector` フックを使用して、巨大な `state` から必要なデータだけを抽出するパターン。
- **役割**: ストアにあるデータの中から、特定のコンポーネントに関連する部分だけを「摘み取る」ピンセットの役割を果たします。
- **実装例**:
  ```tsx
  const paper = usePaperStoreSelector(
    (snapshot) => snapshot.state.paperMap.get(nodeId)
  );
  ```

## 3. 浅い比較と再描画制御 (Equality Checks)

- **概要**: セレクターが返した値が「実際に変わったか」を判定する仕組み。
- **`shallowEqualNodeSelection`**: `PaperNode.tsx` で定義されているカスタム比較関数。
    - `config`, `paper`, `isFocused`, `effectiveAttention` などのプロパティを個別に比較する。
- **最適化の肝**: セレクターが毎回新しいオブジェクトを返しても、この比較関数で「中身が変わっていない」と判定されれば、React の再描画はスキップされます。

## 4. レイアウト情報の分離 (Specialized Contexts)

- **概要**: 論理データ（`PaperStore`）と物理座標（`LayoutContext`）を別々の Context で管理。
- **理由**: ドラッグ操作などで座標（Layout）が高速に変化しても、ドキュメントの論理構造（Store）側の処理に影響を与えず、座標の影響を受けるコンポーネントだけを効率的に更新するため。

## 5. パフォーマンスの考え方：防波堤としてのセレクター

1. **状態の更新**: ユーザーの操作によりグローバルな `state` が更新される。
2. **通知**: ストアがすべてのリスナー（各コンポーネントの `usePaperStoreSelector`）に通知を送る。
3. **フィルタリング**: 各コンポーネントがセレクターを実行し、自分に関係ある値（例：自分の座標）を再計算する。
4. **判定**: 計算結果が前回と同じなら何もしない。変わっていれば自分だけ再描画する。

これにより、「100個のノードがあっても、影響を受けたノードだけが最小限のコストで再描画される」という高いパフォーマンスを実現しています。
