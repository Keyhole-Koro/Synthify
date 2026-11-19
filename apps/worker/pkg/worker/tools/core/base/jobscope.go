package base

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrOutsideJobScope is returned when a tool call names a job, workspace or
// document other than those of the job running the call.
var ErrOutsideJobScope = errors.New("outside the running job's scope")

// ErrInvalidScopeID is returned when a job, workspace or document ID is not a
// single path segment.
var ErrInvalidScopeID = errors.New("invalid scope id")

// maxScopeIDLen bounds the IDs a tool call may name. Real IDs are 26-character
// ULIDs; the bound only rejects absurd values.
const maxScopeIDLen = 128

// JobScope names the job, workspace and document that a tool call acts on.
type JobScope struct {
	JobID       string
	WorkspaceID string
	DocumentID  string
}

// BindJobScope checks the IDs that a model-facing tool call supplies against
// the job running the call, and returns the IDs the tool must use.
//
// The model chooses tool arguments, and an uploaded document can steer the
// model (prompt injection), so the arguments must not reach another tenant's
// workspace or document:
//
//   - an empty ID defaults to the running job's;
//   - an ID that differs from the running job's is refused (ErrOutsideJobScope);
//   - every ID must be a single path segment of at most maxScopeIDLen bytes,
//     because the filesystem tools join the IDs into paths on the shared
//     mount (ErrInvalidScopeID).
//
// The running job is b.Job, which BeginJob sets. b.Job is shared by every job
// in this worker process, so when the call's context carries a different job
// ID (the ADK session ID is the job ID, see jobIDFromContext), a later job has
// replaced b.Job and the caller's own job is looked up in the repository
// instead. Without a running job (direct calls and tests) the IDs are only
// checked to be path segments.
func (b *Context) BindJobScope(ctx context.Context, requested JobScope) (JobScope, error) {
	running, err := b.runningJobScope(ctx)
	if err != nil {
		return JobScope{}, err
	}
	jobID, err := bindScopeID("job_id", requested.JobID, running.JobID)
	if err != nil {
		return JobScope{}, err
	}
	workspaceID, err := bindScopeID("workspace_id", requested.WorkspaceID, running.WorkspaceID)
	if err != nil {
		return JobScope{}, err
	}
	documentID, err := bindScopeID("document_id", requested.DocumentID, running.DocumentID)
	if err != nil {
		return JobScope{}, err
	}
	return JobScope{JobID: jobID, WorkspaceID: workspaceID, DocumentID: documentID}, nil
}

// runningJobScope returns the IDs of the job running this call, or an empty
// JobScope when no job is running.
func (b *Context) runningJobScope(ctx context.Context) (JobScope, error) {
	if b == nil {
		return JobScope{}, nil
	}
	callJobID := jobIDFromContext(ctx)
	if b.Job != nil && (callJobID == "" || callJobID == b.Job.JobID) {
		return JobScope{JobID: b.Job.JobID, WorkspaceID: b.Job.WorkspaceID, DocumentID: b.Job.DocumentID}, nil
	}
	if callJobID == "" {
		return JobScope{}, nil
	}
	if b.Repo == nil {
		return JobScope{}, fmt.Errorf("%w: job %q is not the current job and there is no repository to look it up", ErrOutsideJobScope, callJobID)
	}
	job, err := b.Repo.GetProcessingJob(ctx, callJobID)
	if err != nil {
		return JobScope{}, fmt.Errorf("look up scope of job %s: %w", callJobID, err)
	}
	if job == nil {
		return JobScope{}, fmt.Errorf("%w: job %q not found", ErrOutsideJobScope, callJobID)
	}
	return JobScope{JobID: callJobID, WorkspaceID: job.WorkspaceID, DocumentID: job.DocumentID}, nil
}

// bindScopeID resolves one requested ID against the running job's (empty
// when there is no running job).
func bindScopeID(field, requested, running string) (string, error) {
	if requested != "" {
		if err := checkScopeID(field, requested); err != nil {
			return "", err
		}
	}
	if running == "" {
		return requested, nil
	}
	if err := checkScopeID("running job's "+field, running); err != nil {
		return "", err
	}
	if requested != "" && requested != running {
		return "", fmt.Errorf("%w: %s %q is not the running job's (%q); omit it to use the job's own", ErrOutsideJobScope, field, requested, running)
	}
	return running, nil
}

// checkScopeID reports an error unless id can be used as one path segment.
func checkScopeID(field, id string) error {
	if len(id) > maxScopeIDLen {
		return fmt.Errorf("%w: %s is longer than %d bytes", ErrInvalidScopeID, field, maxScopeIDLen)
	}
	if id == "." || id == ".." || strings.ContainsAny(id, "/\\\x00") {
		return fmt.Errorf("%w: %s %q must be a single path segment", ErrInvalidScopeID, field, id)
	}
	return nil
}
