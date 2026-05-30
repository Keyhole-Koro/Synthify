package llm

import (
	"errors"
	"sync"
	"testing"
)

func TestRecordCallCountsByOutcome(t *testing.T) {
	// Use a model name unique to this test so the global map stays isolated.
	const model = "test-model-outcome"

	got := recordCall(model, nil)
	if got.Total != 1 || got.Succeeded != 1 || got.Failed != 0 {
		t.Fatalf("after one success: %+v", got)
	}

	got = recordCall(model, errors.New("429"))
	if got.Total != 2 || got.Succeeded != 1 || got.Failed != 1 {
		t.Fatalf("after one failure: %+v", got)
	}
}

func TestRecordCallIsConcurrencySafe(t *testing.T) {
	const model = "test-model-concurrent"
	const n = 200

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				recordCall(model, nil)
			} else {
				recordCall(model, errors.New("boom"))
			}
		}(i)
	}
	wg.Wait()

	callCountMu.Lock()
	c := callCounts[model]
	callCountMu.Unlock()
	if c.Total != n || c.Succeeded != n/2 || c.Failed != n/2 {
		t.Fatalf("got %+v, want total=%d succeeded=%d failed=%d", c, n, n/2, n/2)
	}
}
