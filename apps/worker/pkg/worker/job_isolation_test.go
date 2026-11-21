package worker

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	pgvector "github.com/pgvector/pgvector-go"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
	"github.com/synthify/backend/apps/worker/pkg/worker/storage"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// These tests run whole jobs through Worker.Process — the real orchestrator,
// builtin tools and ADK runner — with a fake model, and check that nothing one
// job puts into the worker reaches another job. Cloud Run serves many requests
// per instance and Cloud Tasks dispatches several jobs at once, so two
// tenants' jobs share one worker process.

// isolationTenant is one workspace with one document and a queued job. Its
// document text and glossary term carry markers that must never show up in
// another tenant's job.
type isolationTenant struct {
	name      string
	userID    string
	wsID      string
	docID     string
	jobID     string
	docMarker string // first words of the uploaded document
	term      string // glossary term the model registers in this job
}

// isolationModel is a fake ADK model. It records the system instruction of
// every request (where the orchestrator injects the Working Memory: Document
// Map, brief, glossary, journal) under the job the request belongs to. On a
// job's first turn it calls glossary_register with that job's term; on the
// next turn it answers, which ends the job.
type isolationModel struct {
	terms map[string]string // job ID -> glossary term to register

	// startTogether, when set, holds every job's first turn until all jobs
	// have reached it, so their tool calls and later turns overlap.
	startTogether *startGate

	mu      sync.Mutex
	prompts map[string][]string
}

func newIsolationModel(tenants []*isolationTenant, gate *startGate) *isolationModel {
	terms := make(map[string]string, len(tenants))
	for _, tn := range tenants {
		terms[tn.jobID] = tn.term
	}
	return &isolationModel{terms: terms, startTogether: gate, prompts: map[string][]string{}}
}

func (m *isolationModel) Name() string { return "isolation-fake" }

var jobIDInPrompt = regexp.MustCompile(`job_id: (\S+)`)

func (m *isolationModel) GenerateContent(_ context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		jobID := ""
		for _, c := range req.Contents {
			if c == nil {
				continue
			}
			for _, p := range c.Parts {
				if p == nil {
					continue
				}
				if sm := jobIDInPrompt.FindStringSubmatch(p.Text); sm != nil && jobID == "" {
					jobID = sm[1]
				}
			}
		}
		system := ""
		if req.Config != nil && req.Config.SystemInstruction != nil {
			for _, p := range req.Config.SystemInstruction.Parts {
				if p != nil {
					system += p.Text
				}
			}
		}

		m.mu.Lock()
		m.prompts[jobID] = append(m.prompts[jobID], system)
		turn := len(m.prompts[jobID])
		m.mu.Unlock()

		if turn == 1 {
			m.startTogether.wait()
			yield(&model.LLMResponse{Content: &genai.Content{
				Role: genai.RoleModel,
				Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{
					Name: "glossary_register",
					Args: map[string]any{"term": m.terms[jobID], "definition": "defined in job " + jobID},
				}}},
			}}, nil)
			return
		}
		yield(&model.LLMResponse{Content: genai.NewContentFromText("done", genai.RoleModel)}, nil)
	}
}

func (m *isolationModel) promptsOf(jobID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.prompts[jobID]...)
}

// startGate releases its waiters once n of them have arrived, or after a
// timeout so a broken run fails on its assertions instead of hanging.
type startGate struct {
	mu      sync.Mutex
	arrived int
	n       int
	open    chan struct{}
}

func newStartGate(n int) *startGate {
	return &startGate{n: n, open: make(chan struct{})}
}

func (g *startGate) wait() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.arrived++
	if g.arrived == g.n {
		close(g.open)
	}
	g.mu.Unlock()
	select {
	case <-g.open:
	case <-time.After(10 * time.Second):
	}
}

// toolCallLog records LogToolCall, which the mock store drops.
type toolCallLog struct {
	*mock.Store
	mu    sync.Mutex
	calls []loggedToolCall
}

type loggedToolCall struct {
	jobID, tool, input string
}

func (s *toolCallLog) LogToolCall(_ context.Context, jobID, toolName, inputJSON, _ string, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, loggedToolCall{jobID: jobID, tool: toolName, input: inputJSON})
	return nil
}

func (s *toolCallLog) snapshot() []loggedToolCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]loggedToolCall(nil), s.calls...)
}

// statusLog records the Firestore status writes the jobs make.
type statusLog struct {
	mu     sync.Mutex
	fields []statusWrite
}

type statusWrite struct {
	workspaceID, jobID string
	fields             jobstatus.UpdateFields
}

func (n *statusLog) UpdateFields(_ context.Context, workspaceID, jobID string, f jobstatus.UpdateFields) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.fields = append(n.fields, statusWrite{workspaceID: workspaceID, jobID: jobID, fields: f})
	return nil
}

func (n *statusLog) activityWritesFor(jobID string) []statusWrite {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []statusWrite
	for _, w := range n.fields {
		if w.jobID == jobID && w.fields.LatestActivity != nil {
			out = append(out, w)
		}
	}
	return out
}

func (n *statusLog) snapshot() []statusWrite {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]statusWrite(nil), n.fields...)
}

func (*statusLog) Queued(context.Context, jobstatus.Payload) error        { return nil }
func (*statusLog) Running(context.Context, jobstatus.Payload) error       { return nil }
func (*statusLog) Stage(context.Context, jobstatus.Payload, string) error { return nil }
func (*statusLog) StageProgress(context.Context, jobstatus.Payload, string, int, string) error {
	return nil
}
func (*statusLog) Failed(context.Context, jobstatus.Payload, string) error { return nil }
func (*statusLog) FailedWith(context.Context, jobstatus.Payload, jobstatus.FailureNotification) error {
	return nil
}
func (*statusLog) Completed(context.Context, jobstatus.Payload) error { return nil }
func (*statusLog) CompletedWith(context.Context, jobstatus.Payload, jobstatus.CompletionNotification) error {
	return nil
}

type fixedEmbedder struct{}

func (fixedEmbedder) EmbedText(context.Context, string) (pgvector.Vector, error) {
	return pgvector.NewVector([]float32{0.1, 0.2, 0.3}), nil
}

type isolationHarness struct {
	worker   *Worker
	model    *isolationModel
	store    *toolCallLog
	statuses *statusLog
	fs       *storage.FileSystem
}

// newIsolationTenant creates the tenant's workspace, document and queued job.
// With withSource, the document's original is put on the mount so the
// orchestrator's extract pass can build the Document Map from it.
func newIsolationTenant(t *testing.T, store *mock.Store, fs *storage.FileSystem, name string, withSource bool) *isolationTenant {
	t.Helper()
	ctx := context.Background()
	marker := strings.ToUpper(name) + "-DOC-7f3a"
	tn := &isolationTenant{name: name, userID: "user-" + name, docMarker: marker, term: strings.ToUpper(name) + "-TERM-7f3a"}

	acct, err := store.GetOrCreateAccount(ctx, tn.userID)
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, acct.AccountID, name)
	require.NoError(t, err)
	doc, _, err := store.CreateDocument(ctx, ws.WorkspaceID, tn.userID, name+".txt", "text/plain", 100)
	require.NoError(t, err)
	job, err := store.CreateProcessingJob(ctx, doc.DocumentID, ws.WorkspaceID, tn.userID, appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err)
	tn.wsID, tn.docID, tn.jobID = ws.WorkspaceID, doc.DocumentID, job.JobID

	if withSource {
		dir := fs.SourceDir(tn.wsID, tn.docID)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		body := "# " + name + " report\n" + marker + " is confidential to the " + name + " workspace.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "original.txt"), []byte(body), 0o644))
	}
	return tn
}

func newIsolationHarness(t *testing.T, tenants []*isolationTenant, store *toolCallLog, fs *storage.FileSystem, gate *startGate) *isolationHarness {
	t.Helper()
	statuses := &statusLog{}
	m := newIsolationModel(tenants, gate)
	w, err := NewWorkerWithNotifier(store, store, statuses, m, fixedEmbedder{}, nil, nil, fs, newDiscardLogger(), nil, 30*time.Second)
	require.NoError(t, err)
	return &isolationHarness{worker: w, model: m, store: store, statuses: statuses, fs: fs}
}

func (h *isolationHarness) process(tn *isolationTenant) error {
	return h.worker.Process(context.Background(), ExecutePlanRequest{
		JobID:       tn.jobID,
		JobType:     appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT.String(),
		DocumentID:  tn.docID,
		WorkspaceID: tn.wsID,
		TreeID:      tn.wsID,
		FileURI:     "gs://uploads/" + tn.wsID + "/" + tn.docID,
		Filename:    tn.name + ".txt",
		MimeType:    "text/plain",
	})
}

// requireNoTraceOf fails if any prompt of job tn names the other tenant's
// document or glossary term.
func requireNoTraceOf(t *testing.T, h *isolationHarness, tn, other *isolationTenant) {
	t.Helper()
	prompts := h.model.promptsOf(tn.jobID)
	require.NotEmpty(t, prompts, "job %s never reached the model", tn.jobID)
	for i, p := range prompts {
		require.NotContains(t, p, other.docMarker, "turn %d of %s's job shows %s's document", i+1, tn.name, other.name)
		require.NotContains(t, p, other.term, "turn %d of %s's job shows %s's glossary", i+1, tn.name, other.name)
	}
}

func TestProcess_ConcurrentJobsDoNotShareState(t *testing.T) {
	store := &toolCallLog{Store: mock.NewStore()}
	fs := storage.NewFileSystem(t.TempDir())
	alpha := newIsolationTenant(t, store.Store, fs, "alpha", true)
	bravo := newIsolationTenant(t, store.Store, fs, "bravo", true)
	tenants := []*isolationTenant{alpha, bravo}
	h := newIsolationHarness(t, tenants, store, fs, newStartGate(len(tenants)))

	errs := make([]error, len(tenants))
	var wg sync.WaitGroup
	for i, tn := range tenants {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = h.process(tn)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		require.NoError(t, err, "job %s", tenants[i].jobID)
	}

	for _, pair := range [][2]*isolationTenant{{alpha, bravo}, {bravo, alpha}} {
		tn, other := pair[0], pair[1]
		t.Run(tn.name, func(t *testing.T) {
			prompts := h.model.promptsOf(tn.jobID)
			require.Len(t, prompts, 2, "two model turns per job")
			// The Document Map is built from the job's own document and stays
			// in the prompt for the whole job.
			for i, p := range prompts {
				require.Contains(t, p, tn.docMarker, "turn %d of %s's job lost its own Document Map", i+1, tn.name)
			}
			require.Contains(t, prompts[1], tn.term, "%s's second turn lost its own glossary", tn.name)
			requireNoTraceOf(t, h, tn, other)

			// The job's tool call is logged and reported under its own job.
			var own int
			for _, c := range h.store.snapshot() {
				if strings.Contains(c.input, tn.term) {
					require.Equal(t, tn.jobID, c.jobID, "%s's tool call was logged under another job", tn.name)
					own++
				}
			}
			require.Equal(t, 1, own, "%s's glossary_register call is logged once", tn.name)
			require.NotEmpty(t, h.statuses.activityWritesFor(tn.jobID), "%s's job never reported its activity", tn.name)
			for _, w := range h.statuses.snapshot() {
				if w.jobID == tn.jobID {
					require.Equal(t, tn.wsID, w.workspaceID, "%s's status went to another workspace", tn.name)
				}
			}
		})
	}
}

func TestProcess_NextJobStartsWithEmptyWorkingMemory(t *testing.T) {
	store := &toolCallLog{Store: mock.NewStore()}
	fs := storage.NewFileSystem(t.TempDir())
	alpha := newIsolationTenant(t, store.Store, fs, "alpha", true)
	// bravo's original is missing, so its extract pass fails and the job
	// starts without a Document Map — the case where a previous job's map
	// would otherwise be left in place.
	bravo := newIsolationTenant(t, store.Store, fs, "bravo", false)
	h := newIsolationHarness(t, []*isolationTenant{alpha, bravo}, store, fs, nil)

	require.NoError(t, h.process(alpha))
	require.NoError(t, h.process(bravo))

	require.Contains(t, h.model.promptsOf(alpha.jobID)[1], alpha.term, "alpha's own glossary")
	requireNoTraceOf(t, h, bravo, alpha)
}
