package agents

import "fmt"

// ForJob returns a new Orchestrator for running one job, wired like o.
//
// A worker process runs several jobs at once (Cloud Run serves many requests
// per instance, Cloud Tasks dispatches several jobs at a time) and one after
// another. Everything an Orchestrator keeps for a job — the job's IDs in its
// base.Context, the Working Memory injected into every prompt (Document Map,
// brief, glossary, journal), registered images, the current job ID used for
// tool-call logs, checkpoints and progress, the repeat guard — belongs to that
// job only. Running each job on its own Orchestrator, with its own base.Context
// and builtin tool set, keeps one tenant's document and state out of another
// tenant's prompts and records.
//
// The model, repositories, storage, dynamic tool source and engine, and model
// callbacks are shared; they keep no per-job state.
func (o *Orchestrator) ForJob() (*Orchestrator, error) {
	if o.base == nil {
		return nil, fmt.Errorf("orchestrator has no base context")
	}
	return NewOrchestrator(o.model, o.base.ForJob(), o.repo, o.fs, o.dynamicSource, o.dynamicEngine, o.afterModelCBs)
}
