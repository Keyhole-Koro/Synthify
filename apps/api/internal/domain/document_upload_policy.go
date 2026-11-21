package domain

import (
	"errors"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"
)

var executableDocumentExtensions = map[string]struct{}{
	".apk": {}, ".app": {}, ".bat": {}, ".bin": {}, ".class": {}, ".cmd": {},
	".com": {}, ".dll": {}, ".dmg": {}, ".dylib": {}, ".elf": {}, ".exe": {},
	".jar": {}, ".msi": {}, ".scr": {}, ".so": {},
}

var executableDocumentMIMEs = map[string]struct{}{
	"application/x-dosexec": {}, "application/x-executable": {},
	"application/x-mach-binary": {},
	"application/x-msdownload":  {}, "application/x-msdos-program": {},
	"application/x-msi": {}, "application/x-sharedlib": {},
}

func ValidateDocumentUploadType(filename, mimeType string) error {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	mediaType := normalizeMediaType(mimeType)

	if _, blocked := executableDocumentExtensions[ext]; blocked {
		return ErrUnsupportedDocumentType
	}
	if _, blocked := executableDocumentMIMEs[mediaType]; blocked {
		return ErrUnsupportedDocumentType
	}
	return nil
}

func normalizeMediaType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return value
	}
	return strings.ToLower(mediaType)
}

// ErrUploadLimitReached means the account has already started as many uploads
// as its plan allows in one of the windows below.
var ErrUploadLimitReached = errors.New("upload limit reached")

// The rolling windows that Account.MaxUploadsPerFiveH and
// Account.MaxUploadsPerWeek apply to.
const (
	UploadWindowFiveHours = 5 * time.Hour
	UploadWindowWeek      = 7 * 24 * time.Hour
)

// CheckUploadCount returns ErrUploadLimitReached when the account may not start
// another upload. inFiveHours and inWeek are the uploads the account started in
// the last UploadWindowFiveHours and UploadWindowWeek. Every reserved upload
// counts, confirmed or not, so abandoning an upload does not give it back. A
// limit of zero or less means no limit, as for MaxFileSizeBytes.
func CheckUploadCount(account *Account, inFiveHours, inWeek int64) error {
	if account.MaxUploadsPerFiveH > 0 && inFiveHours >= account.MaxUploadsPerFiveH {
		return fmt.Errorf("%w: at most %d uploads in 5 hours", ErrUploadLimitReached, account.MaxUploadsPerFiveH)
	}
	if account.MaxUploadsPerWeek > 0 && inWeek >= account.MaxUploadsPerWeek {
		return fmt.Errorf("%w: at most %d uploads in 7 days", ErrUploadLimitReached, account.MaxUploadsPerWeek)
	}
	return nil
}
