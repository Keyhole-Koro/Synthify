package domain

import (
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

const planStatusPendingApproval = "pending_approval"

// DeriveLifecycleState computes the document lifecycle state from its latest
// processing job. A document with no job is considered uploaded only.
func DeriveLifecycleState(latestJob *DocumentProcessingJob) appv1.DocumentLifecycleState {
	if latestJob == nil {
		return appv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_UPLOADED
	}
	if latestJob.PlanStatus == planStatusPendingApproval {
		return appv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_PENDING_NORMALIZATION
	}
	switch latestJob.Status {
	case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED,
		appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING:
		return appv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_PROCESSING
	case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_SUCCEEDED:
		return appv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_COMPLETED
	case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_FAILED:
		return appv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_FAILED
	}
	return appv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_UPLOADED
}
