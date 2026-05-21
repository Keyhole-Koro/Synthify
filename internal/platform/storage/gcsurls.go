package storage

import (
	"fmt"
	"net/url"
	"strings"
)

func ValidateBaseURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("parse base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("base URL must include scheme and host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("base URL must not include query or fragment")
	}
	path := strings.Trim(parsed.Path, "/")
	if path != "" {
		return fmt.Errorf("base URL must not include path, got %q", parsed.Path)
	}
	return nil
}

func BuildDocumentUploadURL(baseURL, bucket, workspaceID, objectName string) string {
	root := strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/upload/storage/v1/b/%s/o?uploadType=media&name=%s/%s", root, bucket, workspaceID, objectName)
}

func BuildDocumentSourceURL(baseURL, bucket, workspaceID, documentID string) string {
	root := strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/storage/v1/b/%s/o/%s%%2F%s?alt=media", root, bucket, workspaceID, documentID)
}

func BuildDocumentObjectMetadataURL(baseURL, bucket, workspaceID, documentID string) string {
	root := strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/storage/v1/b/%s/o/%s%%2F%s", root, bucket, workspaceID, documentID)
}
