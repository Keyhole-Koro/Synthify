package base

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/imageref"
	"github.com/synthify/backend/apps/worker/pkg/worker/storage"
)

type stubMemory struct{}

func (stubMemory) RenderForPrompt() string { return "stub" }

// Every field of Context is either a process-wide dependency that ForJob
// shares, or state of one job that ForJob must leave empty. A new field has to
// be added to one of these lists, so it cannot end up shared between jobs by
// accident.
var (
	contextSharedFields = []string{"Repo", "Embedder", "LLM", "FS", "Logger", "Usage", "Status"}
	contextJobFields    = []string{"Memories", "Job", "Images"}
)

func TestForJob_ClassifiesEveryContextField(t *testing.T) {
	known := map[string]bool{}
	for _, f := range append(append([]string{}, contextSharedFields...), contextJobFields...) {
		known[f] = true
	}
	typ := reflect.TypeOf(Context{})
	for i := 0; i < typ.NumField(); i++ {
		if name := typ.Field(i).Name; !known[name] {
			t.Errorf("Context.%s is neither shared nor per-job: add it to ForJob and to one of the lists in this test", name)
		}
	}
}

func TestForJob_SharesDependenciesAndDropsJobState(t *testing.T) {
	usage := NewUsageLimiter(nil, slog.Default())
	parent := &Context{
		FS:       storage.NewFileSystem(t.TempDir()),
		Logger:   slog.Default(),
		Usage:    usage,
		Memories: []PromptMemory{stubMemory{}},
		Job:      &JobContext{JobID: "job-a", WorkspaceID: "ws-a", DocumentID: "doc-a"},
		Images:   imageref.NewRegistry(),
	}
	if _, ok := parent.Images.Register("file-a", "an image of job a"); !ok {
		t.Fatal("register image")
	}

	child := parent.ForJob()

	if child == parent {
		t.Fatal("ForJob returned the same Context")
	}
	pv, cv := reflect.ValueOf(parent).Elem(), reflect.ValueOf(child).Elem()
	for _, f := range contextSharedFields {
		if !reflect.DeepEqual(pv.FieldByName(f).Interface(), cv.FieldByName(f).Interface()) {
			t.Errorf("ForJob did not share %s", f)
		}
	}
	for _, f := range contextJobFields {
		if !cv.FieldByName(f).IsZero() {
			t.Errorf("ForJob carried the previous job's %s over", f)
		}
	}

	// Starting the new job leaves the parent's job untouched.
	child.BeginJob(t.Context(), "job-b", "ws-b", "doc-b")
	if parent.Job.JobID != "job-a" {
		t.Errorf("parent job = %q, want job-a", parent.Job.JobID)
	}
	if _, ok := child.Images.Lookup(1); ok {
		t.Error("the new job sees an image registered by the previous job")
	}
	if _, ok := parent.Images.Lookup(1); !ok {
		t.Error("starting the new job cleared the previous job's images")
	}
	if (*Context)(nil).ForJob() != nil {
		t.Error("ForJob on a nil Context should return nil")
	}
}
