package io

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
	"github.com/synthify/backend/apps/worker/pkg/worker/storage"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

// withZipLimits lowers zipLimits for one test, so small archives can stand in
// for a zip bomb.
func withZipLimits(t *testing.T, entries int, entryBytes, totalBytes uint64, textBytes int) {
	t.Helper()
	saved := zipLimits
	zipLimits.entries, zipLimits.entryBytes, zipLimits.totalBytes, zipLimits.textBytes = entries, entryBytes, totalBytes, textBytes
	t.Cleanup(func() { zipLimits = saved })
}

// extractArchive puts archive on a fresh mount as a document's original and
// extracts it. It returns the extracted/ directory with the result.
func extractArchive(t *testing.T, archive []byte) (string, string, error) {
	t.Helper()
	mount := t.TempDir()
	wsID, docID := "ws_bomb", "doc_bomb"
	sourceDir := filepath.Join(mount, wsID, docID, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "archive.zip"), archive, 0o644))
	b := &base.Context{
		Repo:   mock.NewStore(),
		FS:     storage.NewFileSystem(mount),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Job:    &base.JobContext{WorkspaceID: wsID, DocumentID: docID},
	}
	text, err := Extract(context.Background(), b, "", "application/zip", wsID, docID)
	return filepath.Join(mount, wsID, docID, "extracted"), text, err
}

func binary(n int) string { return string(bytes.Repeat([]byte{0}, n)) }

// An uploaded archive can expand far beyond its own size. Each entry is read
// into memory whole and written to the uploads bucket, and the text of all
// text entries is kept in memory, so an archive past zipLimits would exhaust
// the worker instance (and every other job on it) or the request. Such an
// archive must be refused, before anything is extracted where the declared
// sizes already tell.
func TestExtract_RefusesArchivesBeyondZipLimits(t *testing.T) {
	for _, tc := range []struct {
		name            string
		entries         [][2]string
		nothingExtracts bool
	}{
		{"an entry over the entry limit", [][2]string{{"small.bin", binary(10)}, {"big.bin", binary(1025)}}, true},
		{"entries over the total limit", [][2]string{{"a.bin", binary(1000)}, {"b.bin", binary(1000)}, {"c.bin", binary(100)}}, true},
		{"more files than the file limit", [][2]string{{"1.bin", "1"}, {"2.bin", "2"}, {"3.bin", "3"}, {"4.bin", "4"}}, true},
		{"text over the text limit", [][2]string{{"a.txt", strings.Repeat("a", 800)}, {"b.txt", strings.Repeat("b", 800)}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withZipLimits(t, 3, 1024, 2048, 1500)
			extracted, _, err := extractArchive(t, zipArchive(t, tc.entries))

			require.ErrorIs(t, err, errZipTooLarge)
			if tc.nothingExtracts {
				files, _ := os.ReadDir(extracted)
				assert.Empty(t, files, "nothing is extracted from a refused archive")
			}
		})
	}

	t.Run("an archive within the limits", func(t *testing.T) {
		withZipLimits(t, 3, 1024, 2048, 1500)
		extracted, text, err := extractArchive(t, zipArchive(t, [][2]string{{"notes.txt", "hello"}, {"image.bin", binary(1000)}}))

		require.NoError(t, err)
		assert.Contains(t, text, "hello")
		assert.FileExists(t, filepath.Join(extracted, "notes.txt"))
		assert.FileExists(t, filepath.Join(extracted, "image.bin"))
	})
}

// The production limits stay above what one worker instance (512 MiB, a
// 600-second request) can process, so they refuse only archives that could
// not have been processed anyway.
func TestZipLimits_Defaults(t *testing.T) {
	assert.Equal(t, 10_000, zipLimits.entries)
	assert.Equal(t, uint64(128<<20), zipLimits.entryBytes)
	assert.Equal(t, uint64(1<<30), zipLimits.totalBytes)
	assert.Equal(t, 64<<20, zipLimits.textBytes)
}
