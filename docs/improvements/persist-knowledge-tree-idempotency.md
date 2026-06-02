# persist_knowledge_tree の冪等化

## 背景

`persist_knowledge_tree` は生成した knowledge tree を DB に永続化するツールだが、
**冪等でない**。同じ job が persist を 2 回実行すると、item が重複作成されるか
制約違反で失敗する。

これが worker のエージェントループ timeout 対策（L4: timeout で死んだ job の
自動再開。実装済み・ドキュメントは削除済み、git 履歴参照）の障壁になっている。
再開で persist が再実行
されると tree が壊れるため、現状は「persist 手前で死んだ job だけ再開、persist 以降は
FAILED 確定（手動再投稿）」に留めている。フルの自動再開を可能にするには persist の
冪等化が前提。

## 現状の事実

[persistence.go](../../apps/worker/pkg/worker/tools/builtin/io/persistence.go) /
[item.go](../../apps/worker/pkg/worker/repository/postgres/item.go) より:

- persist は item ごとに **個別 tx** で `CreateDocumentRootItemWithCapability` /
  `CreateStructuredItemWithCapability` を呼ぶ。**persist 全体は 1 tx ではない**
- `document_root` は `document_id` に **UNIQUE 制約**がある
  → 2 回目の root 作成は制約違反
- persist 冒頭に **既存 item チェックも job 単位の cleanup も無い**
- Phase 2（HTML 内の `data-paper-id` を local_id → 永続 ULID に書き換え）も
  別ループ・別更新

つまり「item を数個作って途中で死ぬ」途中状態が現実に起こりうるし、
丸ごと再実行すると確実に壊れる。

## 解決の方向（検討）

### A. job 単位の作り直し（delete → recreate）

persist 冒頭で「この document_id に既存の tree があれば全削除してから作り直す」。

- 利点: 再開ロジックがシンプル。どこから再開しても persist は安全
- 欠点: tree 全体の delete→recreate は重い。item_id が変わるため、Firestore 通知の
  root_id 整合や、tree を参照する他機能（paper-in-paper 等）への影響が読めない

### B. persist 全体を 1 tx 化 + checkpoint スキップ

persist 全体（全 item 作成 + Phase 2）を 1 つの DB transaction にし、
成功なら checkpoint に「persistence done」を記録（仕組みは既存:
[callbacks.go の MarkStageSucceeded](../../apps/worker/pkg/worker/agents/callbacks.go)）。
再開時、persistence checkpoint があれば persist をスキップ。

- 利点: 「途中状態」が無くなる（全 or nothing）ので、checkpoint スキップが安全に成立。
  tree を壊さない。既存 checkpoint 機構の延長
- 欠点: persist の tx 化が必要（今は item ごとにバラバラ）。tx 化で item_id の確定
  タイミングが変わり、Phase 2 の参照解決・root_id 確定の見直しが要る

### C. item に冪等キーを持たせて Upsert

各 item に `(job_id, local_id)` のような冪等キーを持たせ、`ON CONFLICT DO UPDATE`。

- 利点: 部分的な再実行でも重複しない
- 欠点: スキーマ変更（冪等キー列 + UNIQUE）。local_id は job 内でしか一意でない前提の
  確認が要る

## 推奨

**B（1 tx 化 + checkpoint スキップ）** が筋が良さそう。理由:
- 「途中状態を無くす」ことが、再開の安全性と tree を壊さないことを同時に満たす
- 既存の checkpoint 機構（persistence stage は既に checkpoint 対象）に素直に乗る
- A の delete→recreate のような他機能への波及リスクが小さい

ただし persist の tx 化は item_id 確定タイミング・Phase 2 の参照解決に影響するため、
実装前に persist のデータフローを精査する必要がある。

## 関連

- worker エージェントループ timeout 対策の L4-c（フル自動再開）の前提
  （実装済み・ドキュメント削除済み、git 履歴参照）
- [resume-processing-stub.md](resume-processing-stub.md) — ResumeProcessing 本体の実装と統合
- [job-checkpoint-spec](../architecture/job-checkpoint-spec.md) — checkpoint 機構の正式仕様

## 実装前に決めること

1. A / B / C のどれを採るか（推奨 B）
2. B を採る場合、persist の tx 化が Phase 2（HTML 参照書き換え）と root_id 確定に
   与える影響の精査
3. checkpoint 対象の粒度: persistence stage 単位で足りるか、item 単位の進捗も要るか
