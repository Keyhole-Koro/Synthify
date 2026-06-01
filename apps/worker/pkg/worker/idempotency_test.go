package worker

import (
	"testing"

	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

func TestShouldSkipJobStatus(t *testing.T) {
	cases := []struct {
		name   string
		status appv1.JobLifecycleState
		skip   bool
		reason string
	}{
		{
			name:   "queued is processed",
			status: appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED,
			skip:   false,
		},
		{
			name:   "unspecified is processed (default)",
			status: appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_UNSPECIFIED,
			skip:   false,
		},
		{
			name:   "running is skipped (in-flight pass owns it)",
			status: appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING,
			skip:   true,
			reason: "worker.skip_running_job",
		},
		{
			name:   "succeeded is skipped (outputs already exist)",
			status: appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_SUCCEEDED,
			skip:   true,
			reason: "worker.skip_terminal_job",
		},
		{
			name:   "failed is skipped (terminal)",
			status: appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_FAILED,
			skip:   true,
			reason: "worker.skip_terminal_job",
		},
		{
			name:   "retryable is processed (deliberately requeued, must re-run)",
			status: appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RETRYABLE,
			skip:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotSkip, gotReason := shouldSkipJobStatus(tc.status)
			if gotSkip != tc.skip {
				t.Fatalf("skip mismatch: got %v, want %v", gotSkip, tc.skip)
			}
			if gotReason != tc.reason {
				t.Fatalf("reason mismatch: got %q, want %q", gotReason, tc.reason)
			}
		})
	}
}
