package domain

import (
	treev1 "github.com/synthify/backend/internal/gen/synthify/tree/v1"
)

const planStatusPendingApproval = "pending_approval"

// DeriveLifecycleState computes the document lifecycle state from its latest
// processing job. A document with no job is considered uploaded only.
func DeriveLifecycleState(latestJob *DocumentProcessingJob) treev1.DocumentLifecycleState {
	if latestJob == nil {
		return treev1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_UPLOADED
	}
	if latestJob.PlanStatus == planStatusPendingApproval {
		return treev1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_PENDING_NORMALIZATION
	}
	switch latestJob.Status {
	case treev1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED,
		treev1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING:
		return treev1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_PROCESSING
	case treev1.JobLifecycleState_JOB_LIFECYCLE_STATE_SUCCEEDED:
		return treev1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_COMPLETED
	case treev1.JobLifecycleState_JOB_LIFECYCLE_STATE_FAILED:
		return treev1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_FAILED
	}
	return treev1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_UPLOADED
}
