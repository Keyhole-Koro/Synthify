package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"sync"
)

// repeatThreshold is how many consecutive identical (tool, args) calls are
// allowed before the guard intervenes. The first call plus this many repeats
// run normally; the call *after* the threshold is short-circuited with a
// nudge. Kept small: a stuck agent re-issuing the same call at ~4-9s each
// would otherwise eat the Cloud Run request budget long before MaxLLMCalls.
const repeatThreshold = 3

// repeatTracker counts consecutive identical tool calls within a single job.
// "Identical" means same tool name and same arguments; any differing call
// resets the streak, so legitimate progress (different args each step) is
// never throttled. It is created per job and never shared across jobs.
type repeatTracker struct {
	mu      sync.Mutex
	lastKey string
	streak  int
	nudged  map[string]bool // keys already nudged, so we nudge once per stuck key
}

func newRepeatTracker() *repeatTracker {
	return &repeatTracker{nudged: make(map[string]bool)}
}

// observe records a (tool, args) call and reports whether it should be
// short-circuited with a nudge. It returns shouldNudge=true exactly once per
// distinct stuck key once the streak exceeds repeatThreshold, so the agent gets
// a clear signal without the guard itself looping on nudges.
func (t *repeatTracker) observe(toolName string, args map[string]any) (shouldNudge bool, count int) {
	if t == nil {
		return false, 0
	}
	key := callKey(toolName, args)

	t.mu.Lock()
	defer t.mu.Unlock()

	if key == t.lastKey {
		t.streak++
	} else {
		t.lastKey = key
		t.streak = 1
	}

	if t.streak > repeatThreshold && !t.nudged[key] {
		t.nudged[key] = true
		return true, t.streak
	}
	return false, t.streak
}

// callKey is a stable fingerprint of a tool call. Map key ordering in Go is
// non-deterministic, so we marshal with sorted keys to make identical args
// hash identically regardless of iteration order.
func callKey(toolName string, args map[string]any) string {
	h := sha256.New()
	h.Write([]byte(toolName))
	h.Write([]byte{0})
	writeStableJSON(h, args)
	return hex.EncodeToString(h.Sum(nil))
}

func writeStableJSON(h interface{ Write([]byte) (int, error) }, v any) {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		h.Write([]byte{'{'})
		for _, k := range keys {
			b, _ := json.Marshal(k)
			h.Write(b)
			h.Write([]byte{':'})
			writeStableJSON(h, val[k])
		}
		h.Write([]byte{'}'})
	case []any:
		h.Write([]byte{'['})
		for _, e := range val {
			writeStableJSON(h, e)
		}
		h.Write([]byte{']'})
	case string:
		b, _ := json.Marshal(val)
		h.Write(b)
	case float64:
		h.Write([]byte(strconv.FormatFloat(val, 'g', -1, 64)))
	case bool:
		h.Write([]byte(strconv.FormatBool(val)))
	case nil:
		h.Write([]byte("null"))
	default:
		b, _ := json.Marshal(val)
		h.Write(b)
	}
}
