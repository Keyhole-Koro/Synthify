package domain

import (
	"mime"
	"path/filepath"
	"strings"
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
