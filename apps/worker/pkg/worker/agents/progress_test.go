package agents

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
)

// captureNotifier records UpdateFields calls; the other Notifier methods are
// no-ops (only UpdateFields is exercised by progress/activity reporting).
type captureNotifier struct {
	mu     sync.Mutex
	writes []jobstatus.UpdateFields
}

func (c *captureNotifier) UpdateFields(_ context.Context, _, _ string, f jobstatus.UpdateFields) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes = append(c.writes, f)
	return nil
}

func (c *captureNotifier) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.writes)
}

func (c *captureNotifier) Queued(context.Context, jobstatus.Payload) error        { return nil }
func (c *captureNotifier) Running(context.Context, jobstatus.Payload) error       { return nil }
func (c *captureNotifier) Stage(context.Context, jobstatus.Payload, string) error { return nil }
func (c *captureNotifier) StageProgress(context.Context, jobstatus.Payload, string, int, string) error {
	return nil
}
func (c *captureNotifier) Failed(context.Context, jobstatus.Payload, string) error { return nil }
func (c *captureNotifier) FailedWith(context.Context, jobstatus.Payload, jobstatus.FailureNotification) error {
	return nil
}
func (c *captureNotifier) Completed(context.Context, jobstatus.Payload) error { return nil }
func (c *captureNotifier) CompletedWith(context.Context, jobstatus.Payload, jobstatus.CompletionNotification) error {
	return nil
}

func newOrchForProgress(n jobstatus.Notifier) *Orchestrator {
	return &Orchestrator{
		base: &base.Context{
			Status: n,
			Job:    &base.JobContext{JobID: "job-1", WorkspaceID: "ws-1", DocumentID: "doc-1"},
		},
	}
}

func TestReportStageProgress_WritesPercentAndMessage(t *testing.T) {
	n := &captureNotifier{}
	o := newOrchForProgress(n)

	o.reportStageProgress(context.Background(), "briefing")

	if n.count() != 1 {
		t.Fatalf("writes = %d, want 1", n.count())
	}
	w := n.writes[0]
	if w.Progress == nil || *w.Progress != 25 {
		t.Errorf("progress = %v, want 25", w.Progress)
	}
	if w.CurrentStage == nil || *w.CurrentStage != "briefing" {
		t.Errorf("currentStage = %v, want briefing", w.CurrentStage)
	}
	if w.Message == nil || *w.Message == "" {
		t.Error("message should be set")
	}
}

// html_summary is per-item: percent stays fixed while the message advances, so
// the progress bar never rewinds across the multiple item summaries.
func TestReportStageProgress_HTMLSummaryFixedPercentAdvancingMessage(t *testing.T) {
	n := &captureNotifier{}
	o := newOrchForProgress(n)

	o.reportStageProgress(context.Background(), "html_summary")
	o.reportStageProgress(context.Background(), "html_summary")

	if n.count() != 2 {
		t.Fatalf("writes = %d, want 2", n.count())
	}
	for i, w := range n.writes {
		if w.Progress == nil || *w.Progress != 80 {
			t.Errorf("write %d progress = %v, want fixed 80", i, w.Progress)
		}
	}
	if *n.writes[0].Message == *n.writes[1].Message {
		t.Errorf("per-item message should advance, both were %q", *n.writes[0].Message)
	}
}

func TestReportStageProgress_UnknownStageNoWrite(t *testing.T) {
	n := &captureNotifier{}
	o := newOrchForProgress(n)
	o.reportStageProgress(context.Background(), "not_a_stage")
	if n.count() != 0 {
		t.Errorf("writes = %d, want 0 for unknown stage", n.count())
	}
}

// writeActivity throttles: a second call within activityThrottle is dropped.
func TestWriteActivity_Throttles(t *testing.T) {
	n := &captureNotifier{}
	o := newOrchForProgress(n)

	o.writeActivity(context.Background(), "grep_search")
	o.writeActivity(context.Background(), "grep_search") // within throttle window

	if n.count() != 1 {
		t.Fatalf("writes = %d, want 1 (second throttled)", n.count())
	}

	// Force the throttle window to have elapsed, then a new write goes through.
	o.lastActivityWrite.Store(time.Now().Add(-2 * activityThrottle).UnixNano())
	o.writeActivity(context.Background(), "semantic_search")
	if n.count() != 2 {
		t.Fatalf("writes = %d, want 2 after throttle window elapsed", n.count())
	}
}

func TestWriteActivity_UnknownToolNoWrite(t *testing.T) {
	n := &captureNotifier{}
	o := newOrchForProgress(n)
	o.writeActivity(context.Background(), "no_such_tool")
	if n.count() != 0 {
		t.Errorf("writes = %d, want 0 for unmapped tool", n.count())
	}
}
