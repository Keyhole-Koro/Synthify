# 09. Normalization Tools

## Purpose

正規化ツール層は、文書のノード化前にエンコーディング・文字化け・構造の乱れを補正するための**システムグローバルな変換関数群**である。workspace とは無関係に、全ドキュメントに対して適用可能。

管理者（`dev` ロール）が LLM を使って Python スクリプトを生成・検証し、承認後にシステム全体で再利用する。

---

## Target Use Cases

- Shift-JIS を UTF-8 として読み込んだ際の文字化け復元
- UTF-8 の数値コードポイントが文字列として埋め込まれたデータの変換
- 全角数字・全角記号・全角英数字の半角正規化
- 壊れた CSV の列補正
- 日付形式・列名の統一
- PDF 抽出後のノイズ除去

---

## ツールの位置づけ

```
[アップロードされたドキュメント]
        ↓
[エンコーディング・構造の問題を検出]
        ↓
[マッチする approved ツールを自動適用]  ← グローバルに管理された変換関数
        ↓
[正規化済みドキュメント → 以降のパイプラインへ]
```

問題パターンと approved ツールが対応付けられており、同じ問題が検出されたときは自動的に同じツールが再利用される。

---

## Tool Lifecycle

1. 問題ドキュメントまたは問題説明を管理者が入力する
2. LLM がスクリプト案を生成する（`draft`）
3. サンドボックスで dry-run し、差分を確認する
4. **LLM が自動レビューを実行する**
   - 安全性・正確性・dry-run 妥当性・fixture 一致を判定
   - スコア 0.9 以上 → 自動 `approved`（`approved_by: "llm"`）
   - スコア 0.9 未満 or 危険コード検出 → `reviewed` に留まり人間レビュー待ち
5. 管理者が `reviewed` のツールを確認し手動承認（`approved_by: "human"`）
6. `approved` になった時点でドキュメント処理が再開される
7. 同パターンの問題に自動・手動で再利用する
8. 必要に応じて改訂（version を上げる）、廃止（`deprecated`）

---

## Tool Package Layout

```text
tools/
└─ normalize_mojibake_shiftjis/
   ├─ tool.py
   ├─ manifest.yaml
   ├─ README.md
   └─ fixtures/
      ├─ input.txt      ← 文字化けサンプル
      └─ expected.txt   ← 期待する出力
```

---

## Manifest Requirements

- `tool_id`
- `name`
- `version`
- `description`
- `problem_pattern` : このツールが対処する問題パターンの説明（自動マッチングに使用）
- `input_format`
- `output_format`
- `allowed_file_types`
- `timeout_sec`
- `memory_limit_mb`
- `created_by`
- `approved_by`
- `approval_status`

---

## Execution Model

- ツールは Python として実装する
- 実行はサンドボックスコンテナ内で行う
- ネットワークアクセスは原則禁止とする
- 読み取りと書き込みの対象ディレクトリを限定する
- dry-run と本実行を明確に分離する
- `approved` 状態のツールのみ本実行（`APPLY` モード）で使用できる

---

## Approval States

- `draft` : LLM 生成直後
- `reviewed` : dry-run 確認済み・LLM スコアが閾値未満のため人間レビュー待ち
- `approved` : 承認済み（本番適用可能）。`approved_by` が `llm` または `human`
- `deprecated` : 廃止

## LLM 自動レビューの判定基準

| チェック項目 | 内容 |
| --- | --- |
| 安全性 | subprocess・ネットワーク・許可外ファイル書き込みがないか |
| 正確性 | problem_pattern の説明通りの変換をしているか |
| dry-run 妥当性 | 変換差分が意図した内容か、変更量が過大でないか |
| fixture 一致 | input → expected の変換が再現できるか |

スコアが 0.9 以上かつ危険コードなしの場合に自動承認する。

---

## 管理インターフェース

ToolService は `dev` ロールを持つ管理者のみがアクセス可能。フロントの `/dev/stats` 内に管理 UI を配置する。

一般ユーザー（editor/viewer）には公開しない。

---

## Outputs

- 正規化済み成果物
- 標準出力・標準エラー
- 変換差分（行単位）
- 実行時間
- exit status

---

## Risks

- LLM が危険な Python を生成する可能性
- 差分が大きすぎる変換による意図しない破壊
- グローバル適用のため影響範囲が広い

## Mitigations

- サンドボックス実行（ネットワーク・ファイルシステム制限）
- `approved` のみ本番実行可能
- 原本は不変（再処理可能）
- dry-run を標準フロー
- 変換前後差分を保存
- `approved_by` を記録して監査可能にする
