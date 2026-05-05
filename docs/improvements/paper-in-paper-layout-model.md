# paper-in-paper レイアウトモデル（pending）

現在の実装では importance を contentWeight として treemap に渡す比率分割方式を使っている。
以下は次のステップとして残っている設計変更。

## 1. contentSize の絶対値決定

現状の contentWeight は treemap の比率計算にしか使われず、実際のピクセル高さは親の room サイズに依存する。

目標:

```
contentSize(node) = contentHeightMap(node) × f(rawImportance)
```

- `contentHeightMap`（テキストの自然な高さ）は既に state に存在し `computeNodeLayout` に渡されているが未使用
- importance が decay するほど content が物理的に縮む
- importance = 0 で content = 0（header のみ）
