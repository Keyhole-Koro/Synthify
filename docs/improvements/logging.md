# ロギング改善

panic recovery と `fmt.Errorf("%w")` ラップは対応済み。
以下は追加で入れると運用・デバッグが楽になるログの一覧。

---

## P1 — 運用上ないと困る

### 1. API サイドの job lifecycle が見えない

**場所**: `api/internal/service/document.go` — `StartProcessing` / `ResumeProcessing`

Worker が別プロセスの場合、API 側で「ジョブをディスパッチした」事実が記録されない。
今は Worker の `worker.go` にしか job 開始ログがないため、ネットワーク越しに届いたかどうかが追えない。

```go
// job 生成直後
log.Printf("job queued: job=%s doc=%s workspace=%s type=%s", job.JobID, documentID, wsID, jobType)

// dispatcher 呼び出し失敗時
log.Printf("job dispatch failed: job=%s err=%v", job.JobID, err)
```

---

### 2. Worker ConnectHandler にリクエスト受信ログがない

**場所**: `worker/pkg/worker/connect.go` — `ExecuteApprovedPlan` / `GenerateExecutionPlan`

API が送った証拠はあっても、Worker が受け取った証拠がない。
到達確認ができないため、ネットワーク障害とアプリ障害の切り分けが困難。

```go
// ExecuteApprovedPlan 先頭
log.Printf("worker: ExecuteApprovedPlan received: job=%s doc=%s workspace=%s",
    req.Msg.GetJobId(), req.Msg.GetDocumentId(), req.Msg.GetWorkspaceId())

// GenerateExecutionPlan 先頭
log.Printf("worker: GenerateExecutionPlan received: job=%s doc=%s",
    req.Msg.GetJobId(), req.Msg.GetDocumentId())
```

---

### 3. 承認イベントのログがない

**場所**: `api/internal/handler/job.go` — `ApproveJobApproval` / `RejectJobApproval` / `RequestJobApproval`

誰がいつ何を承認・却下したかが DB にしか残らない。
ログに出しておくことで、Cloud Logging 等でのリアルタイム監視・alert 設定が可能になる。

```go
// ApproveJobApproval
log.Printf("approval approved: job=%s approval=%s by=%s", jobID, approvalID, user.ID)

// RejectJobApproval
log.Printf("approval rejected: job=%s approval=%s by=%s reason=%q", jobID, approvalID, user.ID, reason)

// RequestJobApproval
log.Printf("approval requested: job=%s by=%s reason=%q", jobID, user.ID, reason)
```

---

### 4. LLM / tool 使用量の上限到達が黙って通過する

**場所**: `worker/pkg/worker/tools/base/usage.go` — `increment`

上限超過時は `fmt.Errorf` でエラーを返すだけでログに出ない。
エージェントが突然止まった理由がわからない。

```go
// MaxLLMCalls 超過時
log.Printf("usage limit exceeded: job=%s llm_calls=%d/%d", jobID, counters.llmCalls, cap.MaxLLMCalls)
return fmt.Errorf("job %s exceeded LLM call limit: %d > %d", ...)
```

---

## P2 — デバッグ効率に直結する

### 5. Gemini API のレイテンシが記録されない

**場所**: `worker/pkg/worker/llm/gemini.go` — `generate`

LLM 呼び出しは数秒〜数十秒かかるが計測していない。
タイムアウトや遅延の原因追跡に必要。

```go
start := time.Now()
res, err := c.client.Models.GenerateContent(ctx, c.model, contents, config)
log.Printf("gemini: model=%s duration=%s err=%v", c.model, time.Since(start), err)
```

---

### 6. SaveDocumentChunks の保存件数が見えない

**場所**: `shared/repository/postgres/document.go` — `SaveDocumentChunks`

chunking は成功しているのに 0 件保存されるパターンを検出できない。

```go
// tx.Commit() の前
log.Printf("chunks saved: doc=%s count=%d", documentID, len(chunks))
```

---

### 7. Logger が 5xx を目立たせていない

**場所**: `shared/middleware/cors.go` — `Logger`

200 も 500 も同じフォーマットで出力される。
本番では 5xx だけ `ERROR` プレフィックスで出すと alert 設定が楽になる。

```go
if rw.status >= 500 {
    log.Printf("ERROR %s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
} else {
    log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
}
```

---

### 8. Evaluation 結果が記録されない

**場所**: `worker/pkg/worker/worker.go` — `JobEvaluator.Evaluate`

passed / failed / score が DB には書かれるがログに出ない。
Cloud Logging で job の品質傾向を追うのに必要。

```go
log.Printf("evaluation: job=%s passed=%v score=%d findings=%d",
    jobID, result.Passed, result.Score, len(result.Findings))
```

---

## P3 — あると快適

### 9. Document / Workspace 作成の audit log

**場所**: `api/internal/service/document.go` / `service/workspace.go`

```go
log.Printf("document created: doc=%s workspace=%s by=%s filename=%q", ...)
log.Printf("workspace created: ws=%s by=%s name=%q", ...)
```

### 10. スロークエリ警告

**場所**: `shared/middleware/cors.go` — `Logger`

```go
if time.Since(start) > 5*time.Second {
    log.Printf("SLOW %s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
}
```

---

## 対応済み

- panic recovery middleware (`shared/middleware/recover.go`) — API・Worker 両方に適用済み
- `fmt.Errorf("%w", err)` によるエラーコンテキスト付与 — `fetch.go` / `gemini.go` / `worker.go` / `document.go`
