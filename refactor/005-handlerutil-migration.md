# Refactor 005: handlerutil への完全移行

## 現状
`api/internal/handler/handler.go` で `shared/handlerutil` をラップしていますが、一部の古いコードがまだ残っています。

## 課題
- 二重のラップ構造になっており、コードが冗長。
- 共通の HTTP エラー形式などが完全に統一されていない可能性がある。

## 解決策
- `api` 内の各ハンドラから `shared/handlerutil` を直接インポートして使用。
- `api/internal/handler/handler.go` （ラッパーファイル）を最終的に削除。
