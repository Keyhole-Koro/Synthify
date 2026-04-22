# Goal-Driven LLM Worker ガバナンス設計

この文書は、Synthify の LLM worker を `固定パイプライン実行機` から `成果物中心の制約付きエージェント` に拡張する場合の設計原則を整理する。

proto / DB schema / API boundary まで落とした実装仕様は [llm-worker-capability-spec.md](/home/unix/Synthify/docs/llm-worker-capability-spec.md) を参照。

目的は 2 つある。

- worker に十分な自律性を与えて、成果物まで到達しやすくする
- 同時に、権限逸脱、無駄な探索、評価基準のすり替えを防ぐ

この文書では、`成果物(ノード)` を生成・更新・接続する worker を前提に、実務で使える線引きを定義する。

---

## 1. 基本方針

採用するのは `完全自律` ではなく `制約付き自律` である。

worker はゴールから逆算して必要なサブタスクを組み、既存ノードや補助ノードを使って成果物を成立させてよい。  
ただし、以下は worker 単独で決めてはいけない。

- 何をもって成功とするか
- どこまで壊してよいか
- どの外部系を叩いてよいか
- コスト上限をどこに置くか

要するに、worker は `手順の自律` を持ってよいが、`目的の改変権` と `権限境界の変更権` は持たない。

---

## 2. コンポーネント分離

goal-driven worker を 1 つの巨大な agent として扱うより、責務を分けたほうが安定する。

### 2.1 Planner

役割:

- 依頼を成果物単位の plan に落とす
- 必要なノード列、補助ノード、検証ノードを提案する
- 実行前にコストと依存関係を見積もる

持つべき権限:

- read-only で graph と document を参照
- 実行計画案を作る
- approval が必要な操作を列挙する

持つべきでない権限:

- ノードの確定更新
- 外部書き込み
- 評価基準の変更

### 2.2 Worker

役割:

- 許可された plan を実行する
- 既存ノードの再利用、補助ノードの生成、接続修正を行う
- 実行中の失敗を局所的にリカバリする

持つべき権限:

- 許可済みノードへの read / write
- 許可済みツールの実行
- 中間成果物の再生成

持つべきでない権限:

- 成功条件そのものの変更
- 承認不要範囲を自分で拡張すること
- 無関係ノード群への横断変更

### 2.3 Evaluator

役割:

- 成果物が acceptance criteria を満たすか採点する
- groundedness、整合性、欠落、冗長を判定する
- retry すべきか fail すべきかを返す

原則:

- worker と分離する
- できれば別 prompt、別 rule、別 model で判定する
- `生成者が自分を採点する` 状態を避ける

### 2.4 Governor

役割:

- 権限逸脱を止める
- approval 必須操作をブロックする
- 実行 budget、対象範囲、外部副作用を監視する

原則:

- governor は `品質` ではなく `権限と安全性` を見る
- worker の convenience より境界維持を優先する

---

## 3. 権限モデル

worker の権限は `誰が呼んだか` ではなく、`今回の job にどの capability が発行されたか` で決める。

推奨する権限単位は以下。

### 3.1 Resource Scope

どの対象に触れてよいかを定める。

- `workspace_scope`
- `graph_scope`
- `document_scope`
- `node_scope`
- `tool_scope`

最低でも `workspace_id` と `graph_id` は token に束縛する。  
可能なら `allowed_node_ids[]` と `allowed_document_ids[]` も持たせる。

### 3.2 Operation Scope

何をしてよいかを定める。

- `read_graph`
- `read_document`
- `create_node`
- `update_node`
- `create_edge`
- `delete_edge`
- `run_normalization_tool`
- `invoke_llm`
- `emit_plan`
- `emit_eval`

重要なのは `read` と `write` と `execute` を別 capability にすること。

### 3.3 Risk Tier

同じ操作でも危険度が違うので tier を持たせる。

- `tier_0`: read-only
- `tier_1`: 可逆な graph 更新
- `tier_2`: 再計算コストの高い更新
- `tier_3`: 外部副作用を持つ操作

例:

- ノード追加は `tier_1`
- 既存ノードの説明文上書きは `tier_1`
- 大量ノード再配線は `tier_2`
- 外部 API 呼び出しや apply mode の tool 実行は `tier_3`

### 3.4 Approval Policy

各 tier に対して、どこで承認が要るかを固定する。

推奨初期値:

- `tier_0`: 自動許可
- `tier_1`: 自動許可。ただし対象 scope 内のみ
- `tier_2`: plan 承認後のみ許可
- `tier_3`: 個別承認必須

---

## 4. Worker に許す操作の線引き

ここが実務上もっとも重要で、`なんでもする worker` を避ける具体策になる。

### 4.1 自律許可してよい操作

以下は worker が自律的に実行してよい。

- 既存ノードの検索、参照、類似候補の比較
- 同一 job の成果物に必要な補助ノードの追加
- 既存ノードとの hierarchical / semantic edge の追加
- provenance を保った description / summary の再生成
- failed step の再試行
- evaluator 指摘に基づく軽微な修正
- 同一 document 内での chunk 再読込や再要約

条件:

- 対象が `job に紐づく graph/document scope` 内に閉じること
- 評価基準を変更しないこと
- provenance を失わないこと
- rollback 可能な形で記録されること

### 4.2 plan 承認後なら許可してよい操作

以下は planner が plan に明記し、人間または governor が承認したあとに worker が実行する。

- 既存ノード群の大規模マージ
- 多数 edge の張り替え
- level の再編成
- 補助 document や他ノード群を参照する拡張探索
- 再実行コストの高い LLM pass
- dry-run 付き normalization tool の実行

この層では、`なぜ必要か` と `失敗時の戻し方` を plan に含める。

### 4.3 個別承認が必要な操作

以下は worker 単独で実行してはいけない。

- apply mode の normalization tool 実行
- graph 外部の永続ストア書き込み
- 外部システムへの通知、送信、公開
- 既存の accepted node の削除
- 他ユーザー成果物の上書き
- 評価基準、レビュー基準、承認状態の変更

理由は単純で、これらは `成果物を良くする操作` ではなく `責任境界を動かす操作` だからである。

### 4.4 禁止すべき操作

初期設計では明示的に禁止したほうがよい。

- success criteria を worker 自身が再定義すること
- 根拠のない provenance 付与
- 失敗を隠すための evidence 削除
- budget 超過後の継続実行
- 許可されていない tool / model / external endpoint の使用
- approval を bypass するための plan 分割

---

## 5. ノード実行系の権限モデル

Synthify では成果物の中心がノードなので、`node operation` 単位で権限を切るのが自然である。

### 5.1 Node Capability

最低限、worker 実行時に以下の capability を束ねて渡す。

```text
job_id
workspace_id
graph_id
allowed_document_ids[]
allowed_node_ids[]
allowed_operations[]
max_llm_calls
max_tool_runs
max_node_creations
max_edge_mutations
expires_at
```

この token は `今回の job の範囲` を表し、worker は token に無い対象を触れない。

### 5.2 Node Mutation Policy

ノード mutation は 4 種に分けると扱いやすい。

1. `append`
新しい node / edge を追加する。最も安全で、自律許可しやすい。

2. `revise`
既存 node の `description`, `summary`, `alias`, `metadata` を改訂する。  
既存内容を破壊するので provenance と revision log が必要。

3. `relink`
親子関係や semantic edge を張り直す。  
局所変更でも graph 解釈を大きく変えるため、件数閾値を超えたら承認が必要。

4. `remove`
node / edge を削除する。  
最初期は原則禁止、もしくは tombstone 化だけ許可する。

### 5.3 推奨初期設定

初期リリースでは以下が無難である。

- `append`: 自律許可
- `revise`: 自律許可。ただし対象 field を限定
- `relink`: 小規模のみ自律許可
- `remove`: 人間承認必須

`revise` の対象 field はまず以下に絞る。

- `description`
- `summary_html`
- `source_chunk_ids`
- `alias_candidate`

逆に以下は承認なしで触らせないほうがよい。

- `approval_status`
- `canonical_node_id`
- `document ownership`
- `human-authored locked fields`

### 5.4 Lock と Ownership

node には少なくとも論理ロックの概念が必要。

- `system_generated`
- `human_curated`
- `locked`
- `pending_review`

推奨ルール:

- `human_curated` は worker が直接上書きしない
- `locked` は read-only
- `pending_review` は append 型の提案のみ許可

これにより、worker は `提案` はできても `確定破壊` はしにくくなる。

---

## 6. 実行フロー

goal-driven worker を入れる場合の最小フローは以下。

1. user / upstream system が `desired artifact` と `acceptance criteria` を渡す
2. planner が node-level execution plan を生成する
3. governor が plan を capability と policy に照らして検査する
4. worker が許可範囲だけ実行する
5. evaluator が採点する
6. fail なら worker に retry budget 内で差し戻す
7. pass なら persistence して完了する

重要なのは、`plan`, `execution`, `evaluation`, `approval` を同じ LLM 応答に混ぜないこと。

---

## 7. 監査ログ

自律性を上げるなら、監査ログは必須になる。

最低限残すべきもの:

- 入力ゴール
- acceptance criteria
- planner の plan
- worker に発行した capability
- 実行した node mutation 一覧
- tool 実行履歴
- evaluator の採点結果
- 承認イベント
- retry 回数と failure reason

このログが無いと、`なぜそのノードがそうなったか` を後から説明できない。

---

## 8. 設計上の判断

Synthify のように成果物が graph / node であり、途中で補助ノード生成や接続修正が必要になる系では、固定パイプラインだけでは硬い。  
一方で、worker に `成果物のためならなんでもする` を許すと、graph の意味と責任境界が崩れる。

したがって、採るべき設計は以下になる。

- 人間は `目的`, `制約`, `評価基準` を定義する
- planner は node-level plan を作る
- worker は許可済み操作だけで成果物を前に進める
- evaluator は別系統で採点する
- governor は越境と高リスク操作を止める

一言で言えば、強い worker に必要なのは `無制限の自由` ではなく、`十分広い操作権限` と `固定された境界` である。
