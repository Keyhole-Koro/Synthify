package base

// ForJob returns a new Context for running one job.
//
// The new Context shares the process-wide dependencies — repository, embedder,
// LLM client, storage, logger, usage limiter and status notifier — and starts
// with no job, no prompt memories and no registered images. The job's
// orchestrator fills those in (BeginJob, the builtin tool set), so what one job
// keeps there is never seen by another job running in the same process, nor by
// the next job.
//
// The usage limiter is shared on purpose: it keeps its counters per job ID.
func (b *Context) ForJob() *Context {
	if b == nil {
		return nil
	}
	return &Context{
		Repo:     b.Repo,
		Embedder: b.Embedder,
		LLM:      b.LLM,
		FS:       b.FS,
		Logger:   b.Logger,
		Usage:    b.Usage,
		Status:   b.Status,
	}
}
