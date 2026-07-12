package worker

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
)

// FixtureProcessor is a deterministic, local-only replacement for the LLM
// processor. Configuration rejects enabling it outside local/test/dev.
type FixtureProcessor struct {
	repo     Repository
	notifier jobstatus.Notifier
}

func NewFixtureProcessor(repo Repository, notifier jobstatus.Notifier) *FixtureProcessor {
	return &FixtureProcessor{repo: repo, notifier: notifier}
}

func (p *FixtureProcessor) Process(ctx context.Context, req ExecutePlanRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	go func() {
		runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = p.run(runCtx, req)
	}()
	return nil
}

func (p *FixtureProcessor) run(ctx context.Context, req ExecutePlanRequest) error {
	job, err := p.repo.GetProcessingJob(ctx, req.JobID)
	if err != nil {
		return err
	}
	payload := jobstatus.Payload{JobID: req.JobID, JobType: job.JobType.String(), DocumentID: req.DocumentID, WorkspaceID: req.WorkspaceID, TreeID: req.TreeID}
	if err := p.repo.MarkProcessingJobRunning(ctx, req.JobID); err != nil {
		return err
	}
	if err := p.notifier.Running(ctx, payload); err != nil {
		return err
	}
	if err := p.notifier.StageProgress(ctx, payload, "fixture", 35, "Fixture processing"); err != nil {
		return err
	}

	if strings.Contains(strings.ToLower(req.Filename), "e2e-fail") {
		const message = "E2E fixture worker failure"
		_ = p.repo.FailProcessingJob(ctx, req.JobID, message)
		_ = p.notifier.Failed(ctx, payload, message)
		return errors.New(message)
	}
	if strings.Contains(strings.ToLower(req.Filename), "e2e-slow") {
		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	capability, err := p.repo.GetJobCapability(ctx, req.JobID)
	if err != nil {
		return err
	}
	root, err := p.repo.CreateStructuredItemWithCapability(ctx, capability, req.JobID, req.DocumentID, req.WorkspaceID,
		"E2E Fixture Overview", 1, "Deterministic fixture root",
		`<main><h1>E2E Fixture Report</h1><p>Generated deterministically without Gemini.</p></main>`,
		`.fixture-report { color: #4338ca; }`, job.RequestedBy, "", nil)
	if err != nil {
		return err
	}
	child, err := p.repo.CreateStructuredItemWithCapability(ctx, capability, req.JobID, req.DocumentID, req.WorkspaceID,
		"E2E Fixture Detail", 2, "Deterministic fixture child",
		`<section><h2>E2E Fixture Detail</h2><p>Second level paper.</p></section>`, "", job.RequestedBy, root.ItemID, nil)
	if err != nil {
		return err
	}
	if _, err := p.repo.CreateStructuredItemWithCapability(ctx, capability, req.JobID, req.DocumentID, req.WorkspaceID,
		"E2E Fixture Evidence", 3, "Deterministic fixture grandchild",
		`<article><h2>E2E Fixture Evidence</h2><p>Third level paper.</p></article>`, "", job.RequestedBy, child.ItemID, nil); err != nil {
		return err
	}
	if err := p.repo.CompleteProcessingJob(ctx, req.JobID); err != nil {
		return err
	}
	return p.notifier.CompletedWith(ctx, payload, jobstatus.CompletionNotification{TreeChanged: true, StageSummary: []string{"fixture"}})
}

var _ interface {
	Process(context.Context, domain.ExecutePlanRequest) error
} = (*FixtureProcessor)(nil)
