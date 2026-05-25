package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
)

type JobRepository interface {
	MarkProcessingJobRunning(jobID string) bool
	UpdateProcessingJobStage(jobID, stage string) bool
	FailProcessingJob(jobID, errorMessage string) bool
	CompleteProcessingJob(jobID string) bool
}

type PipelineRunner struct {
	stages   []Stage
	jobRepo  JobRepository
	notifier jobstatus.Notifier
	logger   *slog.Logger
}

func NewRunner(jobRepo JobRepository, notifier jobstatus.Notifier, logger *slog.Logger, stages ...Stage) *PipelineRunner {
	return &PipelineRunner{stages: stages, jobRepo: jobRepo, notifier: notifier, logger: logger}
}

func (r *PipelineRunner) Run(ctx context.Context, pctx *PipelineContext) error {
	r.jobRepo.MarkProcessingJobRunning(pctx.JobID)
	if r.notifier != nil {
		r.notifier.Running(ctx, pctx.JobStatusPayload())
	}
	for _, stage := range r.stages {
		r.jobRepo.UpdateProcessingJobStage(pctx.JobID, string(stage.Name()))
		if r.notifier != nil {
			r.notifier.Stage(ctx, pctx.JobStatusPayload(), string(stage.Name()))
		}
		if err := stage.Run(ctx, pctx); err != nil {
			wrapped := fmt.Errorf("stage %s: %w", stage.Name(), err)
			r.jobRepo.FailProcessingJob(pctx.JobID, wrapped.Error())
			if r.notifier != nil {
				r.notifier.Failed(ctx, pctx.JobStatusPayload(), wrapped.Error())
			}
			r.logStageError(ctx, pctx, stage.Name(), wrapped)
			return wrapped
		}
	}
	r.jobRepo.CompleteProcessingJob(pctx.JobID)
	if r.notifier != nil {
		r.notifier.Completed(ctx, pctx.JobStatusPayload())
	}
	return nil
}

func (r *PipelineRunner) logStageError(ctx context.Context, pctx *PipelineContext, stage StageName, err error) {
	attrs := []any{
		"error", err.Error(),
		"job_id", pctx.JobID,
		"workspace_id", pctx.WorkspaceID,
		"document_id", pctx.DocumentID,
		"stage", string(stage),
	}
	if errors.Is(err, domain.ErrCritical) {
		attrs = append(attrs, "severity", "CRITICAL")
		r.logger.Error("pipeline.stage.critical", attrs...)
		return
	}
	r.logger.Error("pipeline.stage.failed", attrs...)
}
