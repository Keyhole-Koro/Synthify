package jobstatus

import (
	"testing"

	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// The DB strings must match the literals in db/queries/documents.sql
// (MarkProcessingJobRunning's `status IN ('queued','running')`, etc.).
func TestLifecycleStateDBValues(t *testing.T) {
	cases := map[appv1.JobLifecycleState]string{
		appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED:    "queued",
		appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING:   "running",
		appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_SUCCEEDED: "succeeded",
		appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_FAILED:    "failed",
		appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RETRYABLE: "retryable",
	}
	for state, want := range cases {
		if got := LifecycleStateToDB(state); got != want {
			t.Errorf("LifecycleStateToDB(%v) = %q, want %q", state, got, want)
		}
		if got := LifecycleStateFromDB(want); got != state {
			t.Errorf("LifecycleStateFromDB(%q) = %v, want %v", want, got, state)
		}
	}
}

func TestJobTypeDBValues(t *testing.T) {
	cases := map[appv1.JobType]string{
		appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT:   "process_document",
		appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT: "reprocess_document",
	}
	for jt, want := range cases {
		if got := JobTypeToDB(jt); got != want {
			t.Errorf("JobTypeToDB(%v) = %q, want %q", jt, got, want)
		}
		if got := JobTypeFromDB(want); got != jt {
			t.Errorf("JobTypeFromDB(%q) = %v, want %v", want, got, jt)
		}
	}
}

// Legacy / unknown values decode to the zero value rather than panicking.
func TestDecodeUnknown(t *testing.T) {
	if got := LifecycleStateFromDB("1"); got != appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_UNSPECIFIED {
		t.Errorf("LifecycleStateFromDB(legacy %q) = %v, want UNSPECIFIED", "1", got)
	}
	if got := JobTypeFromDB(""); got != appv1.JobType_JOB_TYPE_UNSPECIFIED {
		t.Errorf("JobTypeFromDB(%q) = %v, want UNSPECIFIED", "", got)
	}
}
