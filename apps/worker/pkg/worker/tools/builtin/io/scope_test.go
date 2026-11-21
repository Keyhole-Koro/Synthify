package io

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
	"github.com/synthify/backend/apps/worker/pkg/worker/storage"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// sessionCallCtx stands in for the ADK tool context, whose SessionID() is the
// job ID of the agent run making the call.
type sessionCallCtx struct {
	context.Context
	jobID string
}

func (c sessionCallCtx) SessionID() string { return c.jobID }

// putDocFile writes a file under {ws}/{doc}/{sub}/ on the mount: "extracted"
// is where grep_search looks, "source" is where extract_text reads.
func putDocFile(t *testing.T, mount, wsID, docID, sub, name, body string) {
	t.Helper()
	dir := filepath.Join(mount, wsID, docID, sub)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

// sharedMount lays out the running job's document (ws_a/doc_a) and another
// tenant's document (ws_victim/doc_v) on one mount, the way the gcsfuse
// bucket holds every workspace, and returns a context running job_a.
func sharedMount(t *testing.T) *base.Context {
	t.Helper()
	mount := t.TempDir()
	putDocFile(t, mount, "ws_a", "doc_a", "extracted", "notes.md", "- bullet from my own document\nuse -v for verbose output\n")
	putDocFile(t, mount, "ws_a", "doc_a", "source", "original.txt", "my own document")
	putDocFile(t, mount, "ws_victim", "doc_v", "extracted", "secret.txt", "VICTIM-SECRET of another tenant\n")
	putDocFile(t, mount, "ws_victim", "doc_v", "source", "original.txt", "VICTIM-SECRET of another tenant")
	return &base.Context{
		Repo:   mock.NewStore(),
		FS:     storage.NewFileSystem(mount),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Job:    &base.JobContext{JobID: "job_a", WorkspaceID: "ws_a", DocumentID: "doc_a"},
	}
}

func runTool(t *testing.T, ctx context.Context, tool core.Tool, args string) (string, error) {
	t.Helper()
	out, _, err := tool.Run(ctx, json.RawMessage(args))
	return string(out), err
}

func grepMatches(t *testing.T, out string) []GrepMatch {
	t.Helper()
	var res GrepResult
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	return res.Matches
}

// grep_search takes workspace_id and document_id from the model, and an
// uploaded document can steer the model. The IDs must stay those of the job
// running the call; before, they reached any workspace on the shared mount.
func TestGrepTool_StaysInsideRunningJob(t *testing.T) {
	b := sharedMount(t)
	tool, err := NewGrepTool(b)
	require.NoError(t, err)
	ctx := context.Background()

	for name, args := range map[string]string{
		"other workspace":           `{"pattern":"VICTIM-SECRET","workspace_id":"ws_victim","document_id":"doc_v"}`,
		"other document":            `{"pattern":"VICTIM-SECRET","document_id":"doc_v"}`,
		"traversal in workspace_id": `{"pattern":"VICTIM-SECRET","workspace_id":"ws_a/../ws_victim","document_id":"doc_v"}`,
		"traversal in document_id":  `{"pattern":"VICTIM-SECRET","document_id":"../ws_victim/doc_v"}`,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := runTool(t, ctx, tool, args)
			require.Error(t, err)
			assert.NotContains(t, out, "VICTIM-SECRET")
		})
	}

	_, err = runTool(t, ctx, tool, `{"pattern":"VICTIM-SECRET","workspace_id":"ws_victim","document_id":"doc_v"}`)
	assert.ErrorIs(t, err, base.ErrOutsideJobScope)
	_, err = runTool(t, ctx, tool, `{"pattern":"VICTIM-SECRET","document_id":"../ws_victim/doc_v"}`)
	assert.ErrorIs(t, err, base.ErrInvalidScopeID)

	// The job's own document is still searchable, with or without the IDs.
	for _, args := range []string{`{"pattern":"bullet"}`, `{"pattern":"bullet","workspace_id":"ws_a","document_id":"doc_a"}`} {
		out, err := runTool(t, ctx, tool, args)
		require.NoError(t, err, args)
		require.Len(t, grepMatches(t, out), 1, args)
	}
}

// Without a running job the IDs are taken as given, but a "../" segment still
// must not walk into another workspace.
func TestGrepTool_RejectsTraversalWithoutRunningJob(t *testing.T) {
	b := sharedMount(t)
	b.Job = nil
	tool, err := NewGrepTool(b)
	require.NoError(t, err)

	out, err := runTool(t, context.Background(), tool, `{"pattern":"VICTIM-SECRET","workspace_id":"ws_a/../ws_victim","document_id":"doc_v"}`)
	require.ErrorIs(t, err, base.ErrInvalidScopeID)
	assert.NotContains(t, out, "VICTIM-SECRET")
}

// A pattern starting with "-" was parsed by grep as an option: "- bullet"
// failed with "invalid option" and "-v" turned into an inverted match.
func TestGrepTool_PatternStartingWithDash(t *testing.T) {
	b := sharedMount(t)
	tool, err := NewGrepTool(b)
	require.NoError(t, err)

	for pattern, want := range map[string]string{
		"- bullet": "- bullet from my own document",
		"-v":       "use -v for verbose output",
	} {
		t.Run(pattern, func(t *testing.T) {
			args, err := json.Marshal(GrepArgs{Pattern: pattern})
			require.NoError(t, err)
			out, err := runTool(t, context.Background(), tool, string(args))
			require.NoError(t, err)
			matches := grepMatches(t, out)
			require.Len(t, matches, 1)
			assert.Equal(t, want, matches[0].Line)
		})
	}
}

// extract_text reads {workspace_id}/{document_id}/source/ from the shared
// mount, so the same binding applies. The orchestrator's own Extract call is
// not a tool call and is left as is.
func TestExtractTextTool_StaysInsideRunningJob(t *testing.T) {
	b := sharedMount(t)
	tool, err := NewExtractionTool(b)
	require.NoError(t, err)
	ctx := context.Background()

	for name, args := range map[string]string{
		"other workspace":          `{"file_uri":"","workspace_id":"ws_victim","document_id":"doc_v"}`,
		"traversal in document_id": `{"file_uri":"","workspace_id":"ws_a","document_id":"../ws_victim/doc_v"}`,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := runTool(t, ctx, tool, args)
			require.Error(t, err)
			assert.NotContains(t, out, "VICTIM-SECRET")
		})
	}

	out, err := runTool(t, ctx, tool, `{"file_uri":""}`)
	require.NoError(t, err)
	assert.Contains(t, out, "my own document")
}

// The worker shares b.Job across the jobs running in one process. When job B
// begins while job A is still running, A's tool calls must keep working on
// A's document (found through the job ID in the call context) instead of
// failing or drifting into B's.
func TestGrepTool_EarlierJobKeepsItsOwnScopeAfterAnotherJobBegins(t *testing.T) {
	b := sharedMount(t)
	store := mock.NewStore()
	b.Repo = store
	jobA, err := store.CreateProcessingJob(context.Background(), "doc_a", "ws_a", "user_a", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err)
	b.BeginJob(context.Background(), "job_victim", "ws_victim", "doc_v")
	tool, err := NewGrepTool(b)
	require.NoError(t, err)
	callA := sessionCallCtx{Context: context.Background(), jobID: jobA.JobID}

	out, err := runTool(t, callA, tool, `{"pattern":"bullet"}`)
	require.NoError(t, err)
	require.Len(t, grepMatches(t, out), 1)

	out, err = runTool(t, callA, tool, `{"pattern":"VICTIM-SECRET","workspace_id":"ws_victim","document_id":"doc_v"}`)
	require.ErrorIs(t, err, base.ErrOutsideJobScope)
	assert.NotContains(t, out, "VICTIM-SECRET")
}
