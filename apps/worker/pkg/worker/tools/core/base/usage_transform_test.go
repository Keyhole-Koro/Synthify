package base

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

// jobCtx is a context whose SessionID() is the jobID, matching how the worker
// threads jobID through ADK tool contexts (see jobIDFromContext).
type jobCtx struct {
	context.Context
	jobID string
}

func (c jobCtx) SessionID() string { return c.jobID }

// capRepo is a minimal Repository returning a fixed capability for one job.
type capRepo struct {
	Repository
	cap *domain.JobCapability
}

func (r capRepo) GetJobCapability(ctx context.Context, jobID string) (*domain.JobCapability, error) {
	if r.cap != nil && r.cap.JobID == jobID {
		return r.cap, nil
	}
	return nil, domain.ErrNotFound
}

func newLimiterWithCap(t *testing.T, cap *domain.JobCapability) (*UsageLimiter, context.Context) {
	t.Helper()
	l := NewUsageLimiter(capRepo{cap: cap}, slog.New(slog.DiscardHandler))
	ctx := jobCtx{Context: context.Background(), jobID: cap.JobID}
	l.BeginJob(ctx, cap.JobID)
	return l, ctx
}

func TestIncrementTransformCreations_UnderLimit_OK(t *testing.T) {
	l, ctx := newLimiterWithCap(t, &domain.JobCapability{JobID: "job_1", MaxTransformCreations: 3})

	require.NoError(t, l.IncrementTransformCreations(ctx))
	require.NoError(t, l.IncrementTransformCreations(ctx))
	assert.NoError(t, l.IncrementTransformCreations(ctx), "3rd is still within limit")
}

func TestIncrementTransformCreations_OverLimit_Errors(t *testing.T) {
	l, ctx := newLimiterWithCap(t, &domain.JobCapability{JobID: "job_1", MaxTransformCreations: 2})

	require.NoError(t, l.IncrementTransformCreations(ctx))
	require.NoError(t, l.IncrementTransformCreations(ctx))
	err := l.IncrementTransformCreations(ctx)
	assert.Error(t, err, "exceeding MaxTransformCreations must fail (failure budget exhausted)")
}

func TestIncrementTransformRuns_OverLimit_Errors(t *testing.T) {
	l, ctx := newLimiterWithCap(t, &domain.JobCapability{JobID: "job_1", MaxTransformRuns: 1})

	require.NoError(t, l.IncrementTransformRuns(ctx))
	err := l.IncrementTransformRuns(ctx)
	assert.Error(t, err, "exceeding MaxTransformRuns must fail")
}

func TestIncrementTransform_ZeroLimit_Unbounded(t *testing.T) {
	// A zero limit means "not configured" -> no cap, consistent with the
	// existing MaxToolRuns/MaxLLMCalls semantics.
	l, ctx := newLimiterWithCap(t, &domain.JobCapability{JobID: "job_1"})

	for i := 0; i < 50; i++ {
		require.NoError(t, l.IncrementTransformCreations(ctx))
		require.NoError(t, l.IncrementTransformRuns(ctx))
	}
}

func TestIncrementTransform_CreationsAndRunsAreSeparateCounters(t *testing.T) {
	l, ctx := newLimiterWithCap(t, &domain.JobCapability{
		JobID:                 "job_1",
		MaxTransformCreations: 1,
		MaxTransformRuns:      1,
	})

	require.NoError(t, l.IncrementTransformCreations(ctx))
	// Runs counter is independent; first run still ok despite creations maxed.
	assert.NoError(t, l.IncrementTransformRuns(ctx))
	assert.Error(t, l.IncrementTransformCreations(ctx), "creations over its own limit")
}
