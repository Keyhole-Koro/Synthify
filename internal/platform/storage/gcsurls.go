package storage

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// SourceSubdir / ExtractedSubdir mirror the worker's layout
// (see docs/improvements/gcs-storage-layout-redesign.md). The uploaded original
// lives at {ws}/{doc}/source/<name>; the worker expands into {ws}/{doc}/extracted/.
const (
	SourceSubdir    = "source"
	ExtractedSubdir = "extracted"
)

// SourceObjectName returns the object name (within a workspace prefix) for a
// document's uploaded original: "{documentID}/source/original.{ext}". The
// filename is NOT used in the path — only its extension — so user-supplied names
// cannot inject path segments. The original name is preserved in the DB.
func SourceObjectName(documentID, filename string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(filename), "."))
	name := "original"
	if ext != "" {
		name = "original." + ext
	}
	return fmt.Sprintf("%s/%s/%s", documentID, SourceSubdir, name)
}

// SourcePrefix returns the object-name prefix "{documentID}/source/" used to
// locate the uploaded original without knowing its exact extension.
func SourcePrefix(documentID string) string {
	return fmt.Sprintf("%s/%s/", documentID, SourceSubdir)
}

// DocumentPrefix returns "{documentID}/" — the whole document subtree
// (source/ + extracted/), used when deleting a document's objects.
func DocumentPrefix(documentID string) string {
	return documentID + "/"
}

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

// BuildObjectListURL returns the GCS JSON API list URL for objects under the
// given prefix (used against fake-gcs in local dev). prefix is workspace-scoped,
// e.g. "{ws}/{doc}/source/".
func BuildObjectListURL(baseURL, bucket, prefix string) string {
	root := strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/storage/v1/b/%s/o?prefix=%s", root, bucket, url.QueryEscape(prefix))
}
