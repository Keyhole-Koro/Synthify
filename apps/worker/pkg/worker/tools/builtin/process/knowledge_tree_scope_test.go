package process

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

// scopeRecordingLLM records the prompts it is asked to answer.
type scopeRecordingLLM struct {
	prompts []string
}

func (f *scopeRecordingLLM) GenerateStructured(_ context.Context, req llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	f.prompts = append(f.prompts, req.SystemPrompt+"\n"+req.UserPrompt)
	return json.RawMessage(`{"items":[{"local_id":"i1","title":"Generated","level":1,"description":"d","content":"<p>c</p>","parent_local_id":"","child_local_ids":[],"source_chunk_ids":["doc_a_chunk_0"]}]}`), llm.Usage{}, nil
}

func (f *scopeRecordingLLM) GenerateText(context.Context, llm.TextRequest) (string, llm.Usage, error) {
	return "", llm.Usage{}, errors.New("unexpected GenerateText call")
}

// generate_knowledge_tree loads the workspace's tree into its prompt. With a
// model-supplied workspace_id it showed another tenant's node titles and
// descriptions to the model; it must load only the running job's tree.
func TestGenerateKnowledgeTreeTool_StaysInsideRunningJob(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	_, err := store.CreateItem(ctx, "ws_victim", "Victim merger plan", "confidential", "", "user_v")
	require.NoError(t, err)
	_, err = store.CreateItem(ctx, "ws_a", "My overview", "mine", "", "user_a")
	require.NoError(t, err)
	fake := &scopeRecordingLLM{}
	tool, err := NewGenerateKnowledgeTreeTool(&base.Context{
		Repo: store,
		LLM:  fake,
		Job:  &base.JobContext{JobID: "job_a", WorkspaceID: "ws_a", DocumentID: "doc_a"},
	})
	require.NoError(t, err)
	chunks := `"chunks":[{"chunk_index":0,"heading":"Intro","text":"Some text."}]`

	for name, ids := range map[string]string{
		"other workspace": `"job_id":"job_a","workspace_id":"ws_victim","document_id":"doc_a"`,
		"other document":  `"job_id":"job_a","workspace_id":"ws_a","document_id":"doc_v"`,
		"other job":       `"job_id":"job_victim","workspace_id":"ws_a","document_id":"doc_a"`,
		"traversal":       `"job_id":"job_a","workspace_id":"../ws_victim","document_id":"doc_a"`,
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := tool.Run(ctx, json.RawMessage(`{`+ids+`,`+chunks+`}`))
			require.Error(t, err)
		})
	}
	assert.Empty(t, fake.prompts, "a refused call must not reach the model")

	// Omitted IDs are the running job's, so the job's own tree is shown.
	out, _, err := tool.Run(ctx, json.RawMessage(`{`+chunks+`}`))
	require.NoError(t, err)
	assert.Contains(t, string(out), "Generated")
	require.Len(t, fake.prompts, 1)
	assert.Contains(t, fake.prompts[0], "My overview")
	assert.NotContains(t, fake.prompts[0], "Victim merger plan")
}
