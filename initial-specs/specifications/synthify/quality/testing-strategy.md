# Testing Strategy: Synthify の品質保証・テスト計画

## 1. 概要
Synthify は、iframe 内でのコンテンツ表示、重要度ベースの自動レイアウト、Gemini 3 による AI 抽出など、複雑なコンポーネントが組み合わさっている。各レイヤーでの自動テストを強化し、安定したユーザー体験を提供する。

## 2. フロントエンドテスト (frontend/)
`Playwright` と `Vitest` を中心に構成する。

### 2.1. 単体テスト (Vitest)
- **重要度（Importance）モデル**:
  - `useImportanceTick` による減衰率（Decay Rate）の計算。
  - 重要度が閾値を下回った際の `Auto-shrinking` ロジック。
- **データ変換**:
  - API レスポンス（Connect RPC）から `PaperTree` 内部データ構造への変換処理。

### 2.2. E2E テスト (Playwright)
- **Paper の展開・閉じる**:
  - Paper 内のリンクをクリックして、子 Paper が正しく展開されるか。
  - 部屋が満員になった際、古い Paper が自動で縮小または閉じるか。
- **iframe 通信**:
  - `useIframeBridge` を介した、親ウィンドウへの「子 Paper 展開リクエスト」が正しく送出されるか。

## 3. バックエンドテスト (backend/)
`Go Test` を用いたユニットテストと結合テストを行う。

### 3.1. 抽出ロジック (Unit Test)
- **Gemini 3 レスポンスパース**:
  - 不正な JSON 形式が返ってきた際の、エラーハンドリングとリトライ。
  - 抽出された HTML のサニタイズ処理が期待通りか。

### 3.2. Spanner リポジトリ (Integration Test)
- **Paper Tree の一括保存**:
  - 大量（100件以上）の Paper を一括で Spanner に保存できるか。
  - `parent_id` による親子関係の整合性が保たれているか。

## 4. AI 生成品質の評価 (Quality Evaluation)
AI 抽出の精度を定量的に評価する。

- **リンク抽出率**: ソースドキュメント内の概念のうち、いくつの子 Paper リンクが正しく抽出されたか。
- **整合性チェック**: 全ての `data-paper-id` リンクが、実在する Paper ID と紐付いているか（リンク切れ率）。
- **ユーザーフィードバック**: 「この Paper は不要」「ここが抜けている」というフィードバックを収集し、プロンプトの改善に活かす。

## 5. テスト環境 (CI/CD)
GitHub Actions を活用し、プルリクエストごとに以下を実行する。

1. **Linting**: Prettier, ESLint, golangci-lint.
2. **Type Check**: TypeScript compiler (tsc).
3. **Unit Tests**: Vitest, Go test.
4. **Integration Tests**: Cloud Spanner Emulator を用いたバックエンドテスト。
5. **Preview Deploy**: Vercel または Cloud Run によるプレビュー環境の構築と Playwright テストの実行。

## 6. 今後の課題
- **マルチデバイス・ブラウザ対応**: Chrome, Safari, Firefox での iframe 挙動の違いを検証。
- **パフォーマンス計測**: 大規模な Paper Tree（ノード数 500+）を表示した際のメモリ使用量とレンダリング速度のモニタリング。
- **アクセシビリティ (a11y)**: キーボード操作やスクリーンリーダーでの Paper Tree 探索のサポート。
