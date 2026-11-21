package io

import (
	"context"
	"testing"

	pgvector "github.com/pgvector/pgvector-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

// countingEmbedder is a base.Embedder that counts its calls.
type countingEmbedder struct{ calls int }

func (e *countingEmbedder) EmbedText(context.Context, string) (pgvector.Vector, error) {
	e.calls++
	return pgvector.NewVector([]float32{0.1, 0.2, 0.3}), nil
}

// dbJobContext runs job_a (ws_a/doc_a) against a mock store.
func dbJobContext(store *mock.Store, embedder base.Embedder) *base.Context {
	return &base.Context{
		Repo:     store,
		Embedder: embedder,
		Job:      &base.JobContext{JobID: "job_a", WorkspaceID: "ws_a", DocumentID: "doc_a"},
	}
}

// semantic_search returned chunk text for whatever workspace_id the model
// passed. It must search only the running job's workspace.
func TestSemanticSearchTool_StaysInsideRunningJob(t *testing.T) {
	embedder := &countingEmbedder{}
	tool, err := NewSearchTool(dbJobContext(mock.NewStore(), embedder))
	require.NoError(t, err)
	ctx := context.Background()

	_, err = runTool(t, ctx, tool, `{"workspace_id":"ws_victim","query":"secret plans","scope":""}`)
	require.ErrorIs(t, err, base.ErrOutsideJobScope)
	_, err = runTool(t, ctx, tool, `{"workspace_id":"ws_a/../ws_victim","query":"secret plans","scope":""}`)
	require.ErrorIs(t, err, base.ErrInvalidScopeID)
	assert.Zero(t, embedder.calls, "a refused call must not be embedded")

	for _, args := range []string{`{"query":"my notes","scope":""}`, `{"workspace_id":"ws_a","query":"my notes","scope":""}`} {
		_, err := runTool(t, ctx, tool, args)
		require.NoError(t, err, args)
	}
}

// semantic_chunking deletes and replaces the chunks of the document it is
// given. It must not be pointed at another document's chunks.
func TestSemanticChunkingTool_StaysInsideRunningJob(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	embedder := &countingEmbedder{}
	_, _, _, err := ChunkAndSave(ctx, &base.Context{Repo: store, Embedder: embedder}, "doc_v", "", "# Victim\nthe victim's own text")
	require.NoError(t, err)
	victimBefore, err := store.GetDocumentChunks(ctx, "doc_v")
	require.NoError(t, err)

	tool, err := NewChunkingTool(dbJobContext(store, embedder))
	require.NoError(t, err)

	_, err = runTool(t, ctx, tool, `{"document_id":"doc_v","file_id":"","raw_text":"# Injected\nreplacement text"}`)
	require.ErrorIs(t, err, base.ErrOutsideJobScope)
	victimAfter, err := store.GetDocumentChunks(ctx, "doc_v")
	require.NoError(t, err)
	assert.Equal(t, victimBefore, victimAfter, "another document's chunks must stay untouched")

	// An omitted document_id now means the running job's document.
	_, err = runTool(t, ctx, tool, `{"file_id":"","raw_text":"# Mine\nmy own text"}`)
	require.NoError(t, err)
	mine, err := store.GetDocumentChunks(ctx, "doc_a")
	require.NoError(t, err)
	require.NotEmpty(t, mine)
	assert.Equal(t, "doc_a_chunk_0", mine[0].ChunkID)
}

// persist_knowledge_tree looked up the job capability by the model-supplied
// job_id, so naming another job's ID would write with that job's rights.
func TestPersistKnowledgeTreeTool_StaysInsideRunningJob(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	tool, err := NewPersistenceTool(dbJobContext(store, nil))
	require.NoError(t, err)
	items := `[{"local_id":"x","title":"Injected"}]`

	for name, ids := range map[string]string{
		"other job":       `"job_id":"job_victim","workspace_id":"ws_a","document_id":"doc_a"`,
		"other workspace": `"job_id":"job_a","workspace_id":"ws_victim","document_id":"doc_a"`,
		"other document":  `"job_id":"job_a","workspace_id":"ws_a","document_id":"doc_v"`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := runTool(t, ctx, tool, `{`+ids+`,"items":`+items+`}`)
			require.ErrorIs(t, err, base.ErrOutsideJobScope)
		})
	}
	_, err = store.GetTreeByWorkspace(ctx, "ws_victim")
	assert.Error(t, err, "nothing may be created in another workspace")

	// Omitted IDs are the running job's: the capability is looked up for
	// job_a (none is seeded here, so the call stops at that lookup).
	_, err = runTool(t, ctx, tool, `{"items":`+items+`}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job capability not found: job_a")
}
