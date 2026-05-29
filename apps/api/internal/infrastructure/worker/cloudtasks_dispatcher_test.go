package worker

import (
	"errors"
	"testing"
)

func TestTaskID_Deterministic(t *testing.T) {
	// The same (job_id, procedure) pair must hash to the same task id so
	// Cloud Tasks's name-based dedup catches retries.
	a := taskID("job_1", "ExecuteApprovedPlan")
	b := taskID("job_1", "ExecuteApprovedPlan")
	if a != b {
		t.Fatalf("taskID not deterministic: %s vs %s", a, b)
	}
}

func TestTaskID_DistinguishesProcedure(t *testing.T) {
	// GenerateExecutionPlan and ExecuteApprovedPlan are sequential phases
	// of the same job, so they must NOT collide on the dedup name.
	gen := taskID("job_1", "GenerateExecutionPlan")
	exec := taskID("job_1", "ExecuteApprovedPlan")
	if gen == exec {
		t.Fatalf("procedure not part of dedup key: %s", gen)
	}
}

func TestTaskID_DistinguishesJob(t *testing.T) {
	a := taskID("job_1", "ExecuteApprovedPlan")
	b := taskID("job_2", "ExecuteApprovedPlan")
	if a == b {
		t.Fatalf("job_id not part of dedup key: %s", a)
	}
}

func TestTaskID_ValidCloudTasksName(t *testing.T) {
	// Cloud Tasks restricts task names to [A-Za-z0-9_-]{1,500}. The hex
	// digest we use stays inside that alphabet, but assert it explicitly
	// so a refactor that drops the hashing step is caught.
	got := taskID("job with spaces", "ExecuteApprovedPlan")
	if len(got) == 0 || len(got) > 500 {
		t.Fatalf("unexpected task id length: %d", len(got))
	}
	for _, r := range got {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("task id contains non-hex char: %s", got)
		}
	}
}

func TestAudienceFromURL_StripsPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://worker-stage.example/internal/dispatch-job", "https://worker-stage.example"},
		{"https://worker-stage.example", "https://worker-stage.example"},
		{"http://localhost:8081/internal/dispatch-job", "http://localhost:8081"},
		{"http://localhost:8081", "http://localhost:8081"},
		// Trailing slash: behaves like a path of "/".
		{"https://worker-stage.example/", "https://worker-stage.example"},
	}
	for _, c := range cases {
		if got := audienceFromURL(c.in); got != c.want {
			t.Errorf("audienceFromURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsAlreadyExists(t *testing.T) {
	if !isAlreadyExists(errors.New("rpc error: code = AlreadyExists desc = task already exists")) {
		t.Fatalf("AlreadyExists not recognised")
	}
	if isAlreadyExists(errors.New("rpc error: code = Internal")) {
		t.Fatalf("non-AlreadyExists incorrectly recognised")
	}
}
