# Refactor 002: ミドルウェアの整理と共通化

## 現状
内部ワーカー認証用のトークンチェックミドルウェア `RequireWorkerToken` が `worker/pkg/worker/connect.go` に定義されています。

## 課題
- サービス間通信のセキュリティ設定が各サービスに分散している。
- `api` 側でも同様の内部通信ガードが必要になった場合、重複が発生する。

## 解決策
- `shared/middleware` に移動し、共通の認証ガードとして整理。
- 必要に応じて、Firebase Auth との使い分けを明確にする。
