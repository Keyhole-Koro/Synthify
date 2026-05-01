# 巨大ドキュメントのジョブ分割と Router プロキシ

## 背景

1つのジョブが巨大なドキュメントを全部処理する現状設計では、以下の問題が生じる。

- トークン上限に引っかかりやすい
- 話題が多岐にわたる場合、1つの LLM コンテキストで扱うには情報量が多すぎる
- 失敗時に全体をやり直す必要がある

## 設計方針

**Router** というプロキシを worker サービスの中に置き、ドキュメントを分割して複数の子ジョブに割り当てる。

```
Router job (job_123, JOB_TYPE_ROUTE_DOCUMENT)
  ├── chunk + 分割判断
  ├── dispatch → child job_124 (section A)
  ├── dispatch → child job_125 (section B)
  └── 子ジョブ完了後 → merge job (job_126)
```

### Router の配置

**worker サービスの中**（Option B）を採用。

- 「何分割すべきか」の判断に LLM が必要なため API 層では重すぎる
- ADK の 1 agent run 内で複数ジョブを spawn する概念がないため Orchestrator の中には置けない
- Router 自体を `JOB_TYPE_ROUTE_DOCUMENT` という独立したジョブ型として扱う
- 既存の `Orchestrator` を子ジョブでそのまま再利用できる

### 子ジョブ完了の待機方法

**コールバック方式（Option 2）** を採用。

```
1. Router job が子ジョブを dispatch し、DB に親子関係を記録
2. Router job は waiting 状態で終了（ブロックしない）
3. 子ジョブ完了時 → Firestore の jobstatus 通知 or DB の親ジョブを再キュー
4. Router job が再起動 → 子ジョブの状態を確認
5. 全子ジョブ完了 → merge job を dispatch
6. merge job 完了 → Router job が completed になる
```

同期待機（ポーリングしながらブロック）にしない理由：
- Router のゴルーチンが長時間ブロックする
- worker が落ちたら Router ごと死ぬ → 再起動時に子ジョブの結果が残っても Router から再開できない

コールバック方式なら Router が落ちても子ジョブの結果は GCS / DB に残り、再起動時に「子ジョブの状態確認 → 未完なら waiting、完了なら merge」に戻れる。

## 未決定事項

### 1. 分割の基準

- ファイルサイズ（例: 100KB 超えたら分割）
- トピックの多様性（LLM が「これは N 個の話題がある」と判断）
- 章・セクション単位
- これらを組み合わせるか？

### 2. 子ジョブ成果物の統合

- 各子ジョブが独立した subtree を作る → merge job が統合する
- `deduplicate_and_merge` ツールが merge job で担う？
- merge job の job_type は何にするか？

### 3. tree-lifecycle-multi-document との関係

- [tree-lifecycle-multi-document.md](tree-lifecycle-multi-document.md) は「複数ドキュメントの統合」
- こちらは「1ドキュメントの分割処理」
- 実装上は重なる部分がある（merge 処理、親子ジョブの関係など）
- 別物として扱うか、同じ仕組みで統一するか？

### 4. checkpoint との連携

- 子ジョブそれぞれが [json-snapshot-checkpoints.md](json-snapshot-checkpoints.md) の仕組みで snapshot を持つ
- Router job 自体の checkpoint（どの子ジョブを dispatch 済みか）はどう定義するか？

### 5. DB の親子ジョブ関係

- `document_processing_jobs` に `parent_job_id` カラムを追加するか？
- Router が「子ジョブが全部完了したか」を確認するクエリが必要

## 実装前に決めること

未決定事項 1〜5 を固める必要がある。実装は設計確定後。
