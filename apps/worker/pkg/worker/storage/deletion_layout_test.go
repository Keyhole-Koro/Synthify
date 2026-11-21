package storage

import (
	"path/filepath"
	"strings"
	"testing"

	sharedstorage "github.com/synthify/backend/internal/platform/storage"
)

// The API deletes a document's or a job's stored data by object-name prefix
// (internal/platform/storage/prefixes.go). Everything the worker writes for a
// document or a job must sit under one of those prefixes, or deleting the
// document leaves a copy of its content behind.
func TestWorkerWritesUnderDeletionPrefixes(t *testing.T) {
	const mount = "/mnt/gcs/bucket"
	fs := NewFileSystem(mount)

	underPrefix := func(t *testing.T, path, prefix string) {
		t.Helper()
		want := filepath.Join(mount, prefix) + string(filepath.Separator)
		if !strings.HasPrefix(path, want) {
			t.Errorf("%q is not under deletion prefix %q", path, prefix)
		}
	}

	docPrefix := sharedstorage.DocumentObjectPrefix("ws1", "doc1")
	underPrefix(t, filepath.Join(fs.SourceDir("ws1", "doc1"), "original.pdf"), docPrefix)
	underPrefix(t, filepath.Join(fs.ExtractedDir("ws1", "doc1"), "page.txt"), docPrefix)
	underPrefix(t, fs.CachePath("doc1", "grep", "key"), sharedstorage.DocumentCachePrefix("doc1"))
	underPrefix(t, fs.CheckpointPath("job1", "extract"), sharedstorage.JobCheckpointPrefix("job1"))
}
