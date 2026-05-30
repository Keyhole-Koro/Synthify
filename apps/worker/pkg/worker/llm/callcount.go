package llm

import "sync"

// CallCount is a snapshot of how many times this worker process has invoked a
// given model, split by outcome. It exists to answer a single operational
// question — "are we actually calling Vertex often enough to hit the quota, or
// is the 429 coming from something else?" — which the per-call success-only
// logging could not.
type CallCount struct {
	Total     int64
	Succeeded int64
	Failed    int64
}

var (
	callCountMu sync.Mutex
	callCounts  = map[string]*CallCount{}
)

// recordCall increments the process-wide counter for model and returns the
// updated snapshot. err == nil counts as a success.
func recordCall(model string, err error) CallCount {
	callCountMu.Lock()
	defer callCountMu.Unlock()
	c := callCounts[model]
	if c == nil {
		c = &CallCount{}
		callCounts[model] = c
	}
	c.Total++
	if err == nil {
		c.Succeeded++
	} else {
		c.Failed++
	}
	return *c
}
