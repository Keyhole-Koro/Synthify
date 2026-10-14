package domain

import "errors"

var (
	// ErrNotFound indicates that a requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrForbidden indicates an access control or capability violation.
	ErrForbidden = errors.New("forbidden")
	// ErrConflict indicates a state collision such as unique key or foreign key constraint violation.
	ErrConflict = errors.New("conflict")
	// ErrInvariantViolation indicates an unexpected internal inconsistency or software defect.
	ErrInvariantViolation = errors.New("invariant violation")
	// ErrApprovalRequired indicates that the execution plan is awaiting human approval.
	ErrApprovalRequired = errors.New("job execution plan requires approval")
	// ErrPlanRejected indicates that the execution plan was rejected.
	ErrPlanRejected = errors.New("job execution plan was rejected")
	// ErrNotImplemented indicates that an endpoint or feature has not yet been implemented.
	ErrNotImplemented = errors.New("not implemented")
	// ErrFileTooLarge indicates that the uploaded file exceeds maximum size limitations.
	ErrFileTooLarge = errors.New("file exceeds max file size")
	// ErrStorageQuotaExceeded indicates that account storage quota has been exhausted.
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
	// ErrUploadNotConfirmed indicates an attempt to process an unconfirmed document upload.
	ErrUploadNotConfirmed = errors.New("upload is not confirmed")
	// ErrUploadSizeMismatch indicates that the uploaded byte size does not match the reservation.
	ErrUploadSizeMismatch = errors.New("uploaded object size does not match reservation")

	// ErrAgentExecution is a sentinel wrap used by agent-call sites that
	// want their failure to land in the agent_error bucket of the
	// FirestoreJobStatusReason enum. Wrap with fmt.Errorf("...: %w", err,
	// ErrAgentExecution) so errors.Is keeps working alongside the
	// original cause.
	ErrAgentExecution = errors.New("agent execution failed")

	// Severity markers

	// ErrCritical triggers critical alerting when encountered during job runs.
	ErrCritical = errors.New("critical system error") // Triggers CRITICAL alert
	// ErrJobError represents job-level errors triggering standard failure notifications.
	ErrJobError = errors.New("job level error") // Triggers ERROR notification

	// Checkpoint exceptions

	// ErrCheckpointNotFound indicates that no checkpoint exists for the requested stage.
	ErrCheckpointNotFound = errors.New("checkpoint not found")
	// ErrCheckpointInvalid indicates that a checkpoint has an unrecognized schema or corrupt data.
	ErrCheckpointInvalid = errors.New("checkpoint version or format is invalid")
	// ErrCheckpointMismatch indicates that checkpoint inputs do not match current job request parameters.
	ErrCheckpointMismatch = errors.New("checkpoint inputs do not match current request")
)
