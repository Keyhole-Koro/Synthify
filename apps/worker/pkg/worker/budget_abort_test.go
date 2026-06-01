package worker

import (
	"context"
	"testing"
	"time"
)

// isBudgetAbort must fire only when the agent ctx hit its own deadline while the
// parent request ctx is still alive — that is the retryable, self-imposed abort.
// Cloud Run's hard cancel (parent also done) must NOT be treated as one.
func TestIsBudgetAbort(t *testing.T) {
	w := &Worker{agentBudget: 540 * time.Second}

	t.Run("agent deadline, parent alive -> budget abort", func(t *testing.T) {
		parent := context.Background()
		agentCtx, cancel := context.WithTimeout(parent, time.Nanosecond)
		defer cancel()
		<-agentCtx.Done() // ensure the deadline fired
		if !w.isBudgetAbort(parent, agentCtx) {
			t.Error("expected budget abort when agent deadline exceeded and parent alive")
		}
	})

	t.Run("parent also cancelled -> not a budget abort", func(t *testing.T) {
		parent, cancelParent := context.WithCancel(context.Background())
		agentCtx, cancel := context.WithTimeout(parent, time.Nanosecond)
		defer cancel()
		<-agentCtx.Done()
		cancelParent() // Cloud Run hard cancel
		if w.isBudgetAbort(parent, agentCtx) {
			t.Error("Cloud Run hard cancel must not count as a clean budget abort")
		}
	})

	t.Run("agent finished normally -> not a budget abort", func(t *testing.T) {
		parent := context.Background()
		agentCtx, cancel := context.WithCancel(parent)
		cancel() // cancelled, but not DeadlineExceeded
		if w.isBudgetAbort(parent, agentCtx) {
			t.Error("a plain cancel (not deadline) is not a budget abort")
		}
	})

	t.Run("no budget configured -> never a budget abort", func(t *testing.T) {
		w0 := &Worker{agentBudget: 0}
		parent := context.Background()
		agentCtx, cancel := context.WithTimeout(parent, time.Nanosecond)
		defer cancel()
		<-agentCtx.Done()
		if w0.isBudgetAbort(parent, agentCtx) {
			t.Error("with no budget there is no self-imposed abort to detect")
		}
	})
}
