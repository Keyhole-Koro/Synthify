package sourcefile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/synthify/backend/packages/shared/domain"
	"github.com/synthify/backend/packages/shared/storage"
)

func TestLoadReadsFromMount(t *testing.T) {
	mount := t.TempDir()
	docDir := filepath.Join(mount, "ws-1")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docDir, "doc-1"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fs := storage.NewFileSystem(mount)
	file := domain.SourceFile{Filename: "sample.txt", WorkspaceID: "ws-1", DocumentID: "doc-1"}
	if err := Load(context.Background(), fs, &file); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if string(file.Content) != "hello" {
		t.Fatalf("unexpected content: %q", string(file.Content))
	}
}

func TestLoadErrorsWithoutMount(t *testing.T) {
	file := domain.SourceFile{Filename: "sample.txt", WorkspaceID: "ws-1", DocumentID: "doc-1"}
	if err := Load(context.Background(), nil, &file); err == nil {
		t.Fatal("expected error when mount is not configured")
	}
}

func TestLoadErrorsWhenObjectMissing(t *testing.T) {
	fs := storage.NewFileSystem(t.TempDir())
	file := domain.SourceFile{Filename: "sample.txt", WorkspaceID: "ws-1", DocumentID: "missing"}
	if err := Load(context.Background(), fs, &file); err == nil {
		t.Fatal("expected error when source file is absent from the mount")
	}
}
