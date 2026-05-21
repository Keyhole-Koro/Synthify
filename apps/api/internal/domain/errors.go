package domain

import "errors"

var (
	ErrNotFound             = errors.New("not found")
	ErrForbidden            = errors.New("forbidden")
	ErrApprovalRequired     = errors.New("job execution plan requires approval")
	ErrPlanRejected         = errors.New("job execution plan was rejected")
	ErrNotImplemented       = errors.New("not implemented")
	ErrFileTooLarge         = errors.New("file exceeds max file size")
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
	ErrUploadNotConfirmed   = errors.New("upload is not confirmed")
	ErrUploadSizeMismatch   = errors.New("uploaded object size does not match reservation")

	// Severity markers
	ErrCritical = errors.New("critical system error") // Triggers CRITICAL alert
	ErrJobError = errors.New("job level error")       // Triggers ERROR notification

	// Checkpoint exceptions
	ErrCheckpointNotFound = errors.New("checkpoint not found")
	ErrCheckpointInvalid  = errors.New("checkpoint version or format is invalid")
	ErrCheckpointMismatch = errors.New("checkpoint inputs do not match current request")
)
