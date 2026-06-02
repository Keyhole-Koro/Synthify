package sourceio

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	storage "github.com/synthify/backend/apps/worker/pkg/worker/storage"
)

func TestLoadReadsFromMount(t *testing.T) {
	mount := t.TempDir()
	// New layout: the original lives under {ws}/{doc}/source/.
	sourceDir := filepath.Join(mount, "ws-1", "doc-1", "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "original.txt"), []byte("hello"), 0o644); err != nil {
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
