package chat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	chatstatus "github.com/synthify/backend/internal/platform/chat/status"
)

// fakeLLM returns canned section responses in order.
type fakeLLM struct {
	responses []string
	errs      []error
	calls     int
	prompts   []string
}

func (f *fakeLLM) GenerateStructured(_ context.Context, req llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	i := f.calls
	f.calls++
	f.prompts = append(f.prompts, req.UserPrompt)
	usage := llm.Usage{InputTokens: 10, OutputTokens: 5}
	if i < len(f.errs) && f.errs[i] != nil {
		return nil, usage, f.errs[i]
	}
	if i >= len(f.responses) {
		return json.RawMessage(`{"nodes":[],"done":true}`), usage, nil
	}
	return json.RawMessage(f.responses[i]), usage, nil
}

func (f *fakeLLM) GenerateText(context.Context, llm.TextRequest) (string, llm.Usage, error) {
	return "", llm.Usage{}, errors.New("not used")
}

type capturedWriter struct {
	started     bool
	progress    []string
	success     *chatstatus.Success
	failure     *chatstatus.Failure
	startErr    error
	progressErr error
}

func (w *capturedWriter) Start(context.Context, chatstatus.Ref) error {
	w.started = true
	return w.startErr
}

func (w *capturedWriter) Progress(_ context.Context, _ chatstatus.Ref, content string, _ int) error {
	w.progress = append(w.progress, content)
	return w.progressErr
}

func (w *capturedWriter) Succeed(_ context.Context, _ chatstatus.Ref, s chatstatus.Success) error {
	w.success = &s
	return nil
}

func (w *capturedWriter) Fail(_ context.Context, _ chatstatus.Ref, f chatstatus.Failure) error {
	w.failure = &f
	return nil
}

func newGeneratorForTest(l llm.Client) (*Generator, *capturedWriter) {
	w := &capturedWriter{}
	return NewGenerator(l, w, slog.New(slog.NewTextHandler(io.Discard, nil))), w
}

func baseRequest() Request {
	return Request{
		UserID:      "user-1",
		WorkspaceID: "ws-1",
		ChatID:      "chat-1",
		TurnID:      "turn-1",
		PaperID:     "paper-1",
		History:     []Message{{Role: "user", Text: "what is this?"}},
	}
}

func TestRun_SingleSection(t *testing.T) {
	fake := &fakeLLM{responses: []string{`{"nodes":[{"type":"text","value":"hello"}],"done":true}`}}
	g, w := newGeneratorForTest(fake)

	require.NoError(t, g.Run(context.Background(), baseRequest()))

	assert.True(t, w.started)
	assert.Equal(t, 1, fake.calls, "done stops the loop after one call")
	require.NotNil(t, w.success)
	assert.Equal(t, 1, w.success.SectionCount)
	assert.JSONEq(t, `[{"type":"text","value":"hello"}]`, w.success.ContentJSON)
	assert.Equal(t, chatstatus.FirestoreChatTurnFinishReasonStop, w.success.FinishReason)
}

// Each section must publish the whole answer so far, never a delta: onSnapshot
// does not guarantee delivery of every intermediate state.
func TestRun_ProgressCarriesFullContentEachTime(t *testing.T) {
	fake := &fakeLLM{responses: []string{
		`{"nodes":[{"type":"text","value":"one"}],"done":false}`,
		`{"nodes":[{"type":"text","value":"two"}],"done":true}`,
	}}
	g, w := newGeneratorForTest(fake)

	require.NoError(t, g.Run(context.Background(), baseRequest()))

	require.Len(t, w.progress, 2)
	assert.JSONEq(t, `[{"type":"text","value":"one"}]`, w.progress[0])
	assert.JSONEq(t, `[{"type":"text","value":"one"},{"type":"text","value":"two"}]`, w.progress[1],
		"second write repeats the first section")
	require.NotNil(t, w.success)
	assert.Equal(t, 2, w.success.SectionCount)
}

func TestRun_StopsAtSectionCap(t *testing.T) {
	// Never sets done: without a cap this would run forever.
	fake := &fakeLLM{}
	for i := 0; i < maxSections+5; i++ {
		fake.responses = append(fake.responses, `{"nodes":[{"type":"text","value":"x"}],"done":false}`)
	}
	g, w := newGeneratorForTest(fake)

	require.NoError(t, g.Run(context.Background(), baseRequest()))

	assert.Equal(t, maxSections, fake.calls)
	require.NotNil(t, w.success)
	assert.Equal(t, chatstatus.FirestoreChatTurnFinishReasonSectionCap, w.success.FinishReason,
		"a truncated answer must say so")
}

// A section failing after others succeeded keeps what was already produced: a
// partial answer beats discarding the turn.
func TestRun_FailureKeepsEarlierSections(t *testing.T) {
	fake := &fakeLLM{
		responses: []string{`{"nodes":[{"type":"text","value":"kept"}],"done":false}`},
		errs:      []error{nil, errors.New("model exploded")},
	}
	g, w := newGeneratorForTest(fake)

	err := g.Run(context.Background(), baseRequest())

	require.Error(t, err)
	require.Nil(t, w.success)
	require.NotNil(t, w.failure)
	assert.JSONEq(t, `[{"type":"text","value":"kept"}]`, w.failure.ContentJSON)
	assert.Equal(t, 1, w.failure.SectionCount)
	assert.Contains(t, w.failure.ErrorMessage, "model exploded")
}

func TestRun_FailsOnUnparseableSection(t *testing.T) {
	fake := &fakeLLM{responses: []string{`not json at all`}}
	g, w := newGeneratorForTest(fake)

	require.Error(t, g.Run(context.Background(), baseRequest()))
	assert.NotNil(t, w.failure)
}

// If the turn document cannot be created there is nowhere to publish, so
// generating would only burn quota.
func TestRun_DoesNotGenerateWhenStartFails(t *testing.T) {
	fake := &fakeLLM{responses: []string{`{"nodes":[],"done":true}`}}
	g, w := newGeneratorForTest(fake)
	w.startErr = errors.New("firestore down")

	require.Error(t, g.Run(context.Background(), baseRequest()))
	assert.Equal(t, 0, fake.calls, "no model call without somewhere to publish")
}

// A dropped progress write is survivable: the next write carries the full
// content anyway, so the turn should still complete.
func TestRun_SurvivesProgressWriteFailure(t *testing.T) {
	fake := &fakeLLM{responses: []string{`{"nodes":[{"type":"text","value":"x"}],"done":true}`}}
	g, w := newGeneratorForTest(fake)
	w.progressErr = errors.New("transient")

	require.NoError(t, g.Run(context.Background(), baseRequest()))
	assert.NotNil(t, w.success)
}

func TestRun_AccumulatesUsageAcrossSections(t *testing.T) {
	fake := &fakeLLM{responses: []string{
		`{"nodes":[],"done":false}`,
		`{"nodes":[],"done":true}`,
	}}
	g, w := newGeneratorForTest(fake)

	require.NoError(t, g.Run(context.Background(), baseRequest()))

	require.NotNil(t, w.success)
	assert.Equal(t, 20, w.success.Usage.PromptTokens, "10 per call, two calls")
	assert.Equal(t, 10, w.success.Usage.CompletionTokens)
}

func TestRun_EmptyAnswerIsAnArrayNotNull(t *testing.T) {
	fake := &fakeLLM{responses: []string{`{"nodes":[],"done":true}`}}
	g, w := newGeneratorForTest(fake)

	require.NoError(t, g.Run(context.Background(), baseRequest()))

	require.NotNil(t, w.success)
	// "null" would crash the browser, which parses straight into ContentNode[].
	assert.Equal(t, "[]", w.success.ContentJSON)
}

// Later sections need to know what was already said or they repeat the opening.
func TestRun_LaterSectionPromptMentionsEarlierContent(t *testing.T) {
	fake := &fakeLLM{responses: []string{
		`{"nodes":[{"type":"section","title":"Background"}],"done":false}`,
		`{"nodes":[],"done":true}`,
	}}
	g, _ := newGeneratorForTest(fake)

	require.NoError(t, g.Run(context.Background(), baseRequest()))

	require.Len(t, fake.prompts, 2)
	assert.NotContains(t, fake.prompts[0], "Already written")
	assert.Contains(t, fake.prompts[1], "Background", "the follow-up sees what came before")
}

func TestResolveSelectedContext_PinnedAlwaysWinAndModelIDsAreChecked(t *testing.T) {
	req := baseRequest()
	req.PinnedContextIDs = []string{"pinned-1"}
	req.AutoSelectContext = true
	allowed := map[string]bool{"real-1": true}

	got := resolveSelectedContext(req, []string{"real-1", "hallucinated"}, allowed)

	assert.Equal(t, []string{"pinned-1", "real-1"}, got,
		"pinned ids are kept, invented ones dropped")
}

func TestResolveSelectedContext_IgnoresModelWhenAutoSelectOff(t *testing.T) {
	req := baseRequest()
	req.PinnedContextIDs = []string{"pinned-1"}
	req.AutoSelectContext = false

	got := resolveSelectedContext(req, []string{"real-1"}, map[string]bool{"real-1": true})

	assert.Equal(t, []string{"pinned-1"}, got)
}
