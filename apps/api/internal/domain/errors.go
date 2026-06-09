package domain

import "errors"

var (
	ErrNotFound                = errors.New("not found")
	ErrForbidden               = errors.New("forbidden")
	ErrInvalidArgument         = errors.New("invalid argument")     // リクエスト値が不正 (range / enum 違反など)
	ErrConflict                = errors.New("conflict")             // unique / FK 違反など、リクエスト自体は妥当だが現在の状態と衝突
	ErrInvariantViolation      = errors.New("invariant violation")  // 想定外の内部不整合 (バグ相当)。Internal に落とすが区別してロギング
	ErrApprovalRequired        = errors.New("job execution plan requires approval")
	ErrPlanRejected            = errors.New("job execution plan was rejected")
	ErrNotImplemented          = errors.New("not implemented")
	ErrFileTooLarge            = errors.New("file exceeds max file size")
	ErrUnsupportedDocumentType = errors.New("unsupported document type")
	ErrStorageQuotaExceeded    = errors.New("storage quota exceeded")
	ErrUploadNotConfirmed      = errors.New("upload is not confirmed")
	ErrUploadSizeMismatch      = errors.New("uploaded object size does not match reservation")

	// Severity markers
	ErrCritical = errors.New("critical system error") // Triggers CRITICAL alert
	ErrJobError = errors.New("job level error")       // Triggers ERROR notification

	// Checkpoint exceptions
	ErrCheckpointNotFound = errors.New("checkpoint not found")
	ErrCheckpointInvalid  = errors.New("checkpoint version or format is invalid")
	ErrCheckpointMismatch = errors.New("checkpoint inputs do not match current request")
)
