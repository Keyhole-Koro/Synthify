package base

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// toolCallCtx stands in for the ADK tool context, whose SessionID() is the job
// ID of the agent run making the call.
type toolCallCtx struct {
	context.Context
	sessionID string
}

func (c toolCallCtx) SessionID() string { return c.sessionID }

func runningJob() *Context {
	return &Context{Job: &JobContext{JobID: "job_a", WorkspaceID: "ws_a", DocumentID: "doc_a"}}
}

func TestBindJobScope_DefaultsToRunningJob(t *testing.T) {
	got, err := runningJob().BindJobScope(context.Background(), JobScope{})
	require.NoError(t, err)
	assert.Equal(t, JobScope{JobID: "job_a", WorkspaceID: "ws_a", DocumentID: "doc_a"}, got)

	got, err = runningJob().BindJobScope(context.Background(), JobScope{JobID: "job_a", WorkspaceID: "ws_a", DocumentID: "doc_a"})
	require.NoError(t, err)
	assert.Equal(t, JobScope{JobID: "job_a", WorkspaceID: "ws_a", DocumentID: "doc_a"}, got)
}

func TestBindJobScope_RefusesOtherJobsIDs(t *testing.T) {
	for name, req := range map[string]JobScope{
		"workspace": {WorkspaceID: "ws_victim"},
		"document":  {DocumentID: "doc_victim"},
		"job":       {JobID: "job_victim"},
		"all":       {JobID: "job_victim", WorkspaceID: "ws_victim", DocumentID: "doc_victim"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := runningJob().BindJobScope(context.Background(), req)
			require.ErrorIs(t, err, ErrOutsideJobScope)
		})
	}
}

func TestBindJobScope_RefusesIDsThatAreNotOnePathSegment(t *testing.T) {
	bad := []string{"ws_a/../ws_victim", "..", ".", "a/b", `a\b`, "a\x00b", "/abs", strings.Repeat("x", maxScopeIDLen+1)}
	for _, id := range bad {
		// With a running job.
		_, err := runningJob().BindJobScope(context.Background(), JobScope{WorkspaceID: id})
		assert.ErrorIs(t, err, ErrInvalidScopeID, "workspace_id %q", id)
		_, err = runningJob().BindJobScope(context.Background(), JobScope{DocumentID: id})
		assert.ErrorIs(t, err, ErrInvalidScopeID, "document_id %q", id)

		// Without one, the IDs pass through but are still checked.
		_, err = (&Context{}).BindJobScope(context.Background(), JobScope{WorkspaceID: "ws_a", DocumentID: id})
		assert.ErrorIs(t, err, ErrInvalidScopeID, "document_id %q without a job", id)
	}

	// The running job's own IDs are checked too, since they are joined into paths.
	b := &Context{Job: &JobContext{JobID: "job_a", WorkspaceID: "..", DocumentID: "doc_a"}}
	_, err := b.BindJobScope(context.Background(), JobScope{})
	assert.ErrorIs(t, err, ErrInvalidScopeID)
}

func TestBindJobScope_WithoutRunningJobPassesIDsThrough(t *testing.T) {
	req := JobScope{JobID: "job_x", WorkspaceID: "ws_x", DocumentID: "doc_x"}
	for name, b := range map[string]*Context{"nil context": nil, "no job": {}} {
		t.Run(name, func(t *testing.T) {
			got, err := b.BindJobScope(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, req, got)

			got, err = b.BindJobScope(context.Background(), JobScope{})
			require.NoError(t, err)
			assert.Equal(t, JobScope{}, got)
		})
	}
}

func TestBindJobScope_CallFromTheCurrentJobUsesBJob(t *testing.T) {
	ctx := toolCallCtx{Context: context.Background(), sessionID: "job_a"}
	got, err := runningJob().BindJobScope(ctx, JobScope{})
	require.NoError(t, err)
	assert.Equal(t, JobScope{JobID: "job_a", WorkspaceID: "ws_a", DocumentID: "doc_a"}, got)
}

// b.Job is shared by the whole worker process. When job B begins while job A
// is still running, A's tool calls must stay bound to A (looked up by the job
// ID in the call context), neither failing nor drifting into B's scope.
func TestBindJobScope_CallFromAnEarlierJobUsesItsOwnScope(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	jobA, err := store.CreateProcessingJob(ctx, "doc_a", "ws_a", "user_a", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err)
	b := &Context{Repo: store}
	b.BeginJob(ctx, "job_b", "ws_b", "doc_b")

	callA := toolCallCtx{Context: ctx, sessionID: jobA.JobID}
	got, err := b.BindJobScope(callA, JobScope{})
	require.NoError(t, err)
	assert.Equal(t, JobScope{JobID: jobA.JobID, WorkspaceID: "ws_a", DocumentID: "doc_a"}, got)

	got, err = b.BindJobScope(callA, JobScope{JobID: jobA.JobID, WorkspaceID: "ws_a", DocumentID: "doc_a"})
	require.NoError(t, err)
	assert.Equal(t, "doc_a", got.DocumentID)

	_, err = b.BindJobScope(callA, JobScope{WorkspaceID: "ws_b", DocumentID: "doc_b"})
	require.ErrorIs(t, err, ErrOutsideJobScope, "job A must not reach job B's workspace through the shared b.Job")
}

func TestBindJobScope_CallFromAnotherJobWithoutRepositoryFails(t *testing.T) {
	ctx := toolCallCtx{Context: context.Background(), sessionID: "job_other"}
	_, err := runningJob().BindJobScope(ctx, JobScope{WorkspaceID: "ws_a"})
	require.ErrorIs(t, err, ErrOutsideJobScope)
}
