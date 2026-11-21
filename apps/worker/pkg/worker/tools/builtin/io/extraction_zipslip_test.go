package io

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
	"github.com/synthify/backend/apps/worker/pkg/worker/storage"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

// zipArchive builds an in-memory zip whose entries are written in order. Entry
// names are stored verbatim, so a test can plant the hostile names an attacker
// would put in an uploaded archive.
func zipArchive(t *testing.T, entries [][2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e[0])
		require.NoError(t, err)
		_, err = w.Write([]byte(e[1]))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// Zip entry names come from the uploader. Joined onto the extraction directory
// unchecked, "../../../ws_victim/..." climbs out of this document's extracted/
// tree and overwrites another workspace's original on the shared mount (Zip
// Slip). Such entries must be skipped while the safe ones are still extracted.
func TestExtract_ZipEntriesCannotEscapeExtractedDir(t *testing.T) {
	mount := t.TempDir()
	victim := filepath.Join(mount, "ws_victim", "doc_v", "source", "original.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(victim), 0o755))
	require.NoError(t, os.WriteFile(victim, []byte("victim original"), 0o644))

	wsID, docID := "ws_attacker", "doc_zip"
	archive := zipArchive(t, [][2]string{
		{"notes.txt", "safe notes"},
		{"../../../ws_victim/doc_v/source/original.txt", "overwritten by the archive"},
		{"../outside.txt", "escaped one level"},
		{"nested/../../escape.txt", "escaped through a subdirectory"},
		{"/absolute.txt", "absolute name"},
	})
	sourceDir := filepath.Join(mount, wsID, docID, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "archive.zip"), archive, 0o644))

	store := mock.NewStore()
	b := &base.Context{
		Repo:   store,
		FS:     storage.NewFileSystem(mount),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Job:    &base.JobContext{WorkspaceID: wsID, DocumentID: docID},
	}

	rawText, err := Extract(context.Background(), b, "", "application/zip", wsID, docID)
	require.NoError(t, err)

	got, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "victim original", string(got), "a zip entry must not overwrite another workspace's file")
	assert.NoFileExists(t, filepath.Join(mount, wsID, docID, "outside.txt"))
	assert.NoFileExists(t, filepath.Join(mount, wsID, docID, "escape.txt"))

	extracted := filepath.Join(mount, wsID, docID, "extracted")
	assert.FileExists(t, filepath.Join(extracted, "notes.txt"))
	assert.NoFileExists(t, filepath.Join(extracted, "absolute.txt"))
	assert.Contains(t, rawText, "safe notes")
	assert.NotContains(t, rawText, "overwritten by the archive")
	files, err := store.ListDocumentFiles(context.Background(), docID)
	require.NoError(t, err)
	assert.Len(t, files, 1, "only the safe entry gets a document_files record")
}

func TestZipEntryPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "extracted")
	for _, tc := range []struct {
		name string
		want string // relative to dir; "" means rejected
	}{
		{"readme.txt", "readme.txt"},
		{"src/main.go", "src/main.go"},
		{"docs/../readme.txt", "readme.txt"},
		{"./notes.md", "notes.md"},
		{"my report (v2).txt", "my report (v2).txt"},
		{"../escape.txt", ""},
		{"a/../../escape.txt", ""},
		{"../../../ws_victim/doc_v/source/original.txt", ""},
		{"/etc/passwd", ""},
		{"", ""},
		{".", ""},
		{"a/..", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := zipEntryPath(dir, tc.name)
			if tc.want == "" {
				assert.False(t, ok, "entry %q must be rejected, got %q", tc.name, got)
				return
			}
			require.True(t, ok, "entry %q must be accepted", tc.name)
			assert.Equal(t, filepath.Join(dir, tc.want), got)
		})
	}
}
