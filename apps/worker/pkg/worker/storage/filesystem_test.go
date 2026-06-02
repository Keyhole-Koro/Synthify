package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

func TestPathLayout(t *testing.T) {
	fs := NewFileSystem("/mnt")
	if got, want := fs.DocPath("ws", "doc"), filepath.Join("/mnt", "ws", "doc"); got != want {
		t.Errorf("DocPath = %q, want %q", got, want)
	}
	if got, want := fs.SourceDir("ws", "doc"), filepath.Join("/mnt", "ws", "doc", "source"); got != want {
		t.Errorf("SourceDir = %q, want %q", got, want)
	}
	if got, want := fs.ExtractedDir("ws", "doc"), filepath.Join("/mnt", "ws", "doc", "extracted"); got != want {
		t.Errorf("ExtractedDir = %q, want %q", got, want)
	}
}

func TestPathLayout_EmptyWhenUnconfigured(t *testing.T) {
	fs := NewFileSystem("")
	if fs.SourceDir("ws", "doc") != "" || fs.ExtractedDir("ws", "doc") != "" {
		t.Error("expected empty paths when mount is unset")
	}
}

// The reader must find the original under source/ regardless of its exact name,
// since the uploader writes original.{ext} and the worker only knows the doc id.
func TestReadAll_ReadsSingleObjectUnderSource(t *testing.T) {
	mount := t.TempDir()
	sourceDir := filepath.Join(mount, "ws", "doc", "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "original.txt"), []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSystem(mount)
	got, err := fs.ReadAll("ws", "doc")
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "body" {
		t.Errorf("ReadAll = %q, want %q", got, "body")
	}
}

func TestReadAll_NilWhenSourceMissing(t *testing.T) {
	fs := NewFileSystem(t.TempDir())
	got, err := fs.ReadAll("ws", "missing")
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got != nil {
		t.Errorf("ReadAll = %q, want nil for missing source", got)
	}
}

// PopulateSourceFile adopts the on-disk name so MIME sniffing by extension works
// even when the caller only had the document id (Filename left empty).
func TestPopulateSourceFile_AdoptsOnDiskName(t *testing.T) {
	mount := t.TempDir()
	sourceDir := filepath.Join(mount, "ws", "doc", "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "original.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileSystem(mount)
	file := &domain.SourceFile{WorkspaceID: "ws", DocumentID: "doc"} // Filename empty
	ok, err := fs.PopulateSourceFile(file)
	if err != nil || !ok {
		t.Fatalf("PopulateSourceFile ok=%v err=%v", ok, err)
	}
	if file.Filename != "original.txt" {
		t.Errorf("Filename = %q, want adopted on-disk name original.txt", file.Filename)
	}
	if file.MimeType == "" {
		t.Error("expected MIME type sniffed from extension")
	}
}
