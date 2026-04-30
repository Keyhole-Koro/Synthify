# Refactor 003: Shared内部のさらなる重複排除

## 現状
`shared/domain/types.go` や `shared/config/config.go` の中に、`firstNonEmpty` などの小さなユーティリティ関数が個別に定義されています。

## 課題
- 同一パッケージ内で同様の関数が重複している。
- 今回新設した `shared/util` パッケージが十分に活用されていない。

## 解決策
- `shared` 内の全パッケージをスキャンし、`util` パッケージの関数に置き換え。
- パッケージ固有でない汎用関数を徹底的に `util` に集約。
