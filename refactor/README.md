# Refactoring Backlog

今後の改善項目のリストです。各項目の詳細は個別ファイルを参照してください。

- [x] [001: チャンク分割ロジックの共有化](./001-chunking-logic.md)
- [x] [002: ミドルウェアの整理と共通化](./002-middleware-consolidation.md)
- [x] [003: Shared内部のさらなる重複排除](./003-shared-internal-cleanup.md)
- [x] [004: インターフェース分離（疎結合の徹底）](./004-interface-segregation.md)
- [x] [005: handlerutil への完全移行](./005-handlerutil-migration.md)
- [ ] 006: shared/app でのサービス初期化ロジックの共通化
- [ ] 007: ビジネス例外（エラー定義）の標準化と一貫した利用
- [ ] 008: sqlc へのクエリ移行とリポジトリ層の整理
- [ ] 009: ジョブ・ドキュメント状態遷移ロジックの集約
- [ ] 010: テスト用モック・データ生成ファクトリの共有化
