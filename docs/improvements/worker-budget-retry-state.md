# budget timeout を「再試行可能」として表現する状態設計

> **Status: 実装済み（2026-06-01）— 案B採用。** proto に
> `JOB_LIFECYCLE_STATE_RETRYABLE = 5` を追加。worker が budget abort を検出して
> `RequeueProcessingJobForRetry`（status=retryable, retry_count++）を呼び、
> `maxBudgetRetries=3` で上限。`shouldSkipJobStatus` は RETRYABLE を通す。
> api は RETRYABLE→PROCESSING に写像。web は Firestore status 文字列のみ参照し
> RETRYABLE を書かないため変更不要。詳細は下記「案B の実装スケッチ」がそのまま実装。

## 文脈

[worker-agent-loop-timeout.md](worker-agent-loop-timeout.md) の L4-b（persist 手前までの
自動再開）の一部。per-item checkpoint（generate_html_summary を item ごとに
checkpoint）は実装済み。残るのは「**budget timeout で中断した job を、リトライで
再実行可能にする**」経路。

現状の問題:

```
budget timeout
  → L4-a が job を FAILED に確定
    → Cloud Tasks リトライが来ても shouldSkipJobStatus が FAILED を skip
      → ProcessDocument が再実行されない → checkpoint resume も発動しない
```

つまり checkpoint を**書く**ようにはなったが、それを**使う再実行**が起きない。

## 区別すべき2種類の中断

| 種類 | 例 | あるべき扱い |
|---|---|---|
| **再試行で救える中断** | wall-clock budget 超過、429 一時枯渇 | 再実行（checkpoint resume で前進） |
| **本当の失敗** | agent エラー、不正な入力、plan 却下 | FAILED 確定（再実行しても無駄） |

L4-a は今、両方を一律 FAILED にしている。budget timeout だけを切り出して
「再試行可能」にしたい。

## 状態をどう表現するか（設計の核心）

`JobLifecycleState` enum は現在 `UNSPECIFIED / QUEUED / RUNNING / SUCCEEDED / FAILED`
の5値（[job.proto](../../contracts/connectrpc/synthify/app/v1/job.proto)）。
「再試行可能」を表す状態が無い。

### 案A: QUEUED に戻す

budget timeout 時に job を QUEUED に戻す。`shouldSkipJobStatus` は QUEUED を通すので
リトライが再実行できる。

- 利点: proto 変更なし。UI 上も QUEUED は RUNNING と同じく `PROCESSING` に見える
  （[document_lifecycle.go](../../apps/api/internal/domain/document_lifecycle.go)）ので
  UX は自然。最小工数
- 欠点: **意味論が濁る。** QUEUED は本来「まだ一度も着手していない」状態。
  「budget timeout で再キューされた（checkpoint が一部ある）」と区別できない。
  オブザーバビリティ上「この QUEUED は初回か再試行か」が状態だけでは分からない
  （RetryCount で補えるが、状態の意味が二重になる）

### 案B: RETRYABLE 状態を新設（proto 拡張）

`JOB_LIFECYCLE_STATE_RETRYABLE = 5` を足す。budget timeout 時にこの状態にし、
`shouldSkipJobStatus` は RETRYABLE を通す。

- 利点: **状態が意味通り。** 「中断したが再試行待ち」が一目で分かる。
  オブザーバビリティ・UI で初回処理中と区別できる。FAILED とも QUEUED とも違う、
  正しい第3の終端でない中間状態
- 欠点: proto 変更 → 生成コード再生成、api / worker / web の switch 文すべてに
  ケース追加（[document_lifecycle.go](../../apps/api/internal/domain/document_lifecycle.go)
  ほか）。`shouldSkipJobStatus` も対応。工数は案Aより明確に大きい

### 案C: status は RUNNING のまま + stale 検知

job は RUNNING を維持し、`shouldSkipJobStatus` を「RUNNING かつ最終更新から N 分
以上 → stale とみなして通す」に変える。

- 利点: proto 変更なし
- 欠点: budget abort（自分で止めた、即再試行可）と Cloud Run ハードキャンセル
  （ctx 死亡、別インスタンスが処理中かもしれない）を時間だけで区別することになり、
  競合判定が脆い。「N 分」のチューニングも環境依存。ユーザーは案 B 寄りの明示性を希望

## 推奨

**案B（RETRYABLE 状態の新設）。** 「工数がかかってもきれいな設計に」という方針に沿う。
budget timeout は「失敗」でも「未着手」でもない固有の状態であり、それを enum で
明示するのが最も正直。状態だけでオブザーバビリティ・UI・再実行判定が完結する。

## 案B の実装スケッチ

1. **proto**: `JOB_LIFECYCLE_STATE_RETRYABLE = 5` を追加 → コード生成
2. **worker**: budget timeout（`agentCtx.Err()==DeadlineExceeded` かつ親 ctx 生存）を
   検出し、`failJob` ではなく新メソッド（job を RETRYABLE にし、RetryCount++）を呼ぶ。
   `return err` で internal_dispatch が 500 → Cloud Tasks リトライ
3. **shouldSkipJobStatus**: RETRYABLE を「通す」（skip しない）。
   再実行で checkpoint resume が効き、済み item / stage をスキップ
4. **リトライ上限**: 再実行のたびに RetryCount を見て、N 回（例 3）超えたら
   RETRYABLE にせず FAILED 確定（Cloud Tasks の max_attempts と二重の安全網）
5. **api / web**: RETRYABLE を `DOCUMENT_LIFECYCLE_STATE_PROCESSING` に写像
   （UI 上は「処理中」のまま。ユーザーには中断が見えない）
6. **lifecycle / mock**: 新状態に対応

## 区別ロジック（worker 側）

```
ProcessDocument がエラー
  ├─ budget timeout（agentCtx.Err()==DeadlineExceeded && ctx.Err()==nil）
  │    ├─ RetryCount < maxRetries → RETRYABLE + RetryCount++、return err（500→retry）
  │    └─ RetryCount >= maxRetries → failJob（FAILED 確定）
  └─ それ以外（本当の失敗 / 親 ctx も死亡）
       → failJob（FAILED 確定）
```

親 ctx も死んでいる（Cloud Run ハードキャンセル）場合は budget abort と区別し、
従来通り failJob（L4-a の止血が効く）。

## 実装前に決めること

1. 案 A / B / C（推奨 B）
2. maxRetries の値（案: 3。Cloud Tasks max_attempts と整合させる）
3. RETRYABLE 中の Firestore 通知をどうするか（PROCESSING のまま無通知か、
   「再試行中」を出すか）
