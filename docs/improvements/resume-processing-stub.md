# ResumeProcessing がダミー job_id を返すだけ

## 場所
`api/internal/handler/document.go:143` — `ResumeProcessing()`

## 問題
```go
JobId: "job_resume_" + doc.DocumentID,
```
ハードコードされたダミー job_id を返しているだけで、実際の再開ロジックが存在しない。DB にジョブが作成されず、worker への dispatch も行われない。

## 修正方針
`StartProcessing(forceReprocess=false)` に相当するフローを呼び出し、既存の中断ジョブを再キューするか新規ジョブを作成する。
