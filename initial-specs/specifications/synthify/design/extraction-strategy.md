# Extraction Strategy: Gemini 3 による Paper Tree の抽出

## 1. 抽出の概要
ユーザーから提供されたソースドキュメント（PDF、Markdown、Webサイトのテキスト等）から、Synthify で利用可能な「Paper Tree」構造を抽出する。Gemini 3 の高いコンテキスト理解能力を活用し、単なる要約ではなく、再帰的に探索可能なインタラクティブな構造を生成する。

## 2. 抽出対象のデータ構造
各 Paper は以下のフィールドを持つ。

- `id`: 一意の ID（プロンプト内で生成させる）。
- `title`: Paper のタイトル。
- `description`: ホバー時に表示される短い概要（100文字程度）。
- `content_html`: iframe 内で表示される HTML 本文。
- `child_ids`: この Paper からリンクされている子 Paper の ID リスト。

## 3. Gemini 3 へのプロンプト戦略

### 3.1. プロンプトの構成
1. **Role**: 複雑な知識を階層構造に整理し、インタラクティブな教育コンテンツを作成する専門家。
2. **Context**: 元となるソーステキスト。
3. **Task**: テキストを複数の「Paper」に分割し、それらを再帰的な親子関係で結ぶ。
4. **Constraint**:
   - HTML 内には、子 Paper へのリンクとして `<a data-paper-id="child-id">リンクテキスト</a>` を必ず含めること。
   - `child_ids` リストと、HTML 内の `data-paper-id` は完全に一致させること。
   - スタイルはインライン CSS ではなく、提供される標準クラス（後述）を使用すること。
5. **Output Format**: JSON 形式（`Array<Paper>`）。

### 3.2. HTML 生成のガイドライン
- **セマンティック HTML**: `<h1>`, `p`, `ul`, `li`, `blockquote` 等を使用。
- **インタラクティブリンク**: `data-paper-id` 属性を持つ `<a>` タグのみを許容する（外部 `href` は原則禁止、または `target="_blank"` を強制）。
- **サニタイズ**: `script`, `style`, `onclick` 等のイベントハンドラ属性の生成を禁止。

## 4. 抽出アルゴリズム

### 段階的抽出 (Incremental Extraction)
非常に長いドキュメントの場合、一度に全体を抽出すると精度が落ちるため、以下のステップを踏む。

1. **Step 1: Root & First Level**: 全体の構造を把握し、ルート Paper と主要な章（第1階層の子 Paper）を抽出。
2. **Step 2: Recursive Detail**: 各子 Paper の内容を元に、さらに詳細な子 Paper（第2階層以降）を抽出する。
3. **Step 3: Verification**: 全ての `data-paper-id` が `papers` リスト内に実在するかを検証し、欠損があれば補完プロンプトを実行。

## 5. 整合性チェック (Validation)
抽出されたデータに対して、バックエンド側で以下のバリデーションを行う。

- **リンク整合性**: `child_ids` に含まれる ID が `papers` コレクション内に存在するか。
- **HTML サニタイズ**: DOMPurify 等を用いて、意図しないスクリプトを排除。
- **循環参照チェック**: 無限ループを避けるため、親子関係が木構造（DAG）になっているかを確認。

## 6. 今後の検討事項
- **マルチモーダル抽出**: Gemini 3 の画像理解能力を使い、図解やグラフも画像として切り出し、Paper 内に配置する。
- **差分更新**: ソースドキュメントが更新された際、既存の `id` を維持したまま最小限の変更で再抽出する手法。
