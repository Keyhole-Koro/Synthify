# Synthify: グラフから Paper-in-Paper への移行計画案

## 1. プロジェクトの目的
「GCP Graph System」におけるノード・エッジのグラフ構造を、「Paper-in-Paper」ライブラリを利用した「再帰的な部屋と紙」の構造に置き換え、より能動的で空間的な読書・探索体験（Synthify）を提供する。

## 2. コアコンセプトの変更
- **データ構造**:
  - 旧: Node-Edge グラフ（Spanner Graph）
  - 新: Paper Tree（再帰的な Parent-Child 構造）
- **可視化**:
  - 旧: React Flow による 2D マップ
  - 新: Paper-in-Paper（部屋/Room 単位のネスト構造、重要度ベースの自動レイアウト）
- **コンテンツ表示**:
  - 各 Paper は独立した iframe で描画され、CSS/JS が隔離される。
  - 文中の `<a data-paper-id="xxx">` リンクをクリックすることで、部屋の中に子 Paper が展開される。

## 3. 実装・設計の変更方針

### データモデル (`domain/data-model.md`)
- `nodes` テーブルを `papers` に統合・拡張。
  - `parent_id` (STRING): 親 Paper ID。
  - `child_ids` (ARRAY<STRING>): 子 Paper ID の配列。
  - `content_html` (STRING): iframe 内に表示する HTML。
  - `description` (STRING): ホバー時に表示される概要。
- `edges` テーブルの役割を、Paper 内の `child_ids` および HTML 内の `data-paper-id` リンクに置き換える。

### API 契約 (`contract/api-spec.md`)
- `GetGraph` から `GetPaperTree` / `GetPaperRoom` への変更。
- 部屋単位での遅延読み込み（Lazy loading）をサポートするインターフェースの検討。

### UI/UX 仕様 (`design/frontend.md`)
- `vender/paper-in-paper` ライブラリの統合。
- 重要度（Importance）モデルの導入:
  - アクセス頻度や経過時間に応じて各 Paper のサイズ・展開状態を自動制御。
  - スペース不足時に重要度の低いノードを自動縮小（Auto-shrinking）。

## 4. 進捗状況と次のステップ

### 完了した作業
- [x] 既存仕様 (`gcp-graph-system`) の読み込みと構造分析。
- [x] `paper-in-paper` ライブラリの挙動・レイアウト仕様の把握。
- [x] `initial-specs/specifications/synthify/` ディレクトリの構築。
- [x] `product/overview.md`: Synthify のビジョン定義。
- [x] `product/requirements.md`: Paper Tree 抽出や iframe 通信の機能要件定義。
- [x] `domain/data-model.md`: `papers` テーブルと重要度管理のデータ定義。
- [x] `design/architecture.md`: システム全体のサーバレス構成とフロントエンド統合。
- [x] `contract/api-spec.md`: Connect RPC を用いた PaperTree 取得 API の定義（エッジを排除しツリーに特化）。

### 次のフェーズ: 詳細設計とプロトタイピング
次のステップとして、より具体的な「どう作るか」の設計文書を整備します。

1. **`design/extraction-strategy.md` (最優先)**:
   - Gemini 3 を用いて、テキストからどうやって「タイトル、概要、HTML本文、子IDリスト」を一度に、かつ整合性を保って抽出するか。
   - 再帰的な構造を一段ずつ抽出するのか、一気に全体を生成するのかの戦略。
   - 文中リンク (`data-paper-id`) と子 ID リストの完全な同期を保証するプロンプトエンジニアリング。
2. **`implementation/backend-structure.md`**:
   - Go (Connect RPC) でのディレクトリ構成と、Spanner Graph をツリー走査にどう活用するか。
   - クリーンアーキテクチャに基づいたサービス分割（Extraction Service, Storage Service, Tree View Service）。
3. **`quality/testing-strategy.md`**:
   - iframe 間の通信や、重要度ベースの自動縮小が正しく動くかをどうテストするか。
   - Playwright を用いた E2E テスト。
   - 重要度（Importance）モデルの単体テスト（減衰計算、閾値判定）。
4. **プロトタイプの実装**:
   - `frontend/` 配下で `vender/paper-in-paper` を使った最小構成の Synthify 画面を立ち上げる。
   - `demoData.ts` を実際の API レスポンスに近い形式に繋ぎ込む。
   - Next.js (App Router) との統合。


---

## 5. 設計上の懸念・検討事項 (High-priority)
- **HTML 生成の安全性**: Gemini が生成する HTML に悪意あるスクリプトが含まれないよう、サニタイズ処理の徹底（iframe 隔離はあるが、それでも必要）。
- **抽出の整合性**: 本文内の `data-paper-id` が `child_ids` に存在しない「リンク切れ」をどう防ぐか。
- **重要度のチューニング**: ユーザーが「勝手に閉じるな！」と感じないための、減衰率（Decay Rate）のパラメータ調整。
