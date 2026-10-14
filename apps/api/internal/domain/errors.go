package domain

import "errors"

var (
	// ErrNotFound indicates that a requested domain resource was not found.
	ErrNotFound = errors.New("not found")
	// ErrForbidden indicates that the authenticated user lacks permission to access the resource.
	ErrForbidden = errors.New("forbidden")
	// ErrInvalidArgument indicates invalid request arguments or constraint violations.
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrConflict indicates state conflict such as unique constraint or foreign key collision.
	ErrConflict = errors.New("conflict")
	// ErrInvariantViolation indicates an unexpected internal logic or invariant failure.
	ErrInvariantViolation = errors.New("invariant violation")
	// ErrApprovalRequired indicates that a job execution plan requires human approval before proceeding.
	ErrApprovalRequired = errors.New("job execution plan requires approval")
	// ErrPlanRejected indicates that a proposed job execution plan was rejected by a reviewer.
	ErrPlanRejected = errors.New("job execution plan was rejected")
	// ErrNotImplemented indicates an unimplemented operation or feature stub.
	ErrNotImplemented = errors.New("not implemented")
	// ErrFileTooLarge indicates that an uploaded document exceeds the maximum permitted byte size.
	ErrFileTooLarge = errors.New("file exceeds max file size")
	// ErrUnsupportedDocumentType indicates that an uploaded document has an unsupported extension or MIME type.
	ErrUnsupportedDocumentType = errors.New("unsupported document type")
	// ErrStorageQuotaExceeded indicates that a workspace or account has exceeded its allocated storage limit.
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
	// ErrUploadNotConfirmed indicates that upload confirmation was attempted on an unfulfilled reservation.
	ErrUploadNotConfirmed = errors.New("upload is not confirmed")
	// ErrUploadSizeMismatch indicates that the uploaded object byte size differs from the reserved upload size.
	ErrUploadSizeMismatch = errors.New("uploaded object size does not match reservation")

	// Severity markers

	// ErrCritical denotes a critical system condition triggering high-priority alerts.
	ErrCritical = errors.New("critical system error")
	// ErrJobError denotes an execution error within a specific job.
	ErrJobError = errors.New("job level error")

	// Checkpoint exceptions

	// ErrCheckpointNotFound indicates that a requested pipeline stage checkpoint could not be found.
	ErrCheckpointNotFound = errors.New("checkpoint not found")
	// ErrCheckpointInvalid indicates an invalid or unsupported checkpoint format/version.
	ErrCheckpointInvalid = errors.New("checkpoint version or format is invalid")
	// ErrCheckpointMismatch indicates that checkpoint inputs do not match current stage parameters.
	ErrCheckpointMismatch = errors.New("checkpoint inputs do not match current request")
)
