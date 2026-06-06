package sourceio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

func TestLoadFetchesExternalImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png; charset=binary")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	}))
	defer srv.Close()

	file := domain.SourceFile{URI: srv.URL}
	// fs is nil: an http URI must not touch the mount.
	if err := Load(context.Background(), nil, &file); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(file.Content) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("unexpected content: %q", file.Content)
	}
	if file.MimeType != "image/png" {
		t.Fatalf("MimeType = %q, want image/png", file.MimeType)
	}
}

func TestLoadRejectsNonImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>nope</html>"))
	}))
	defer srv.Close()

	file := domain.SourceFile{URI: srv.URL}
	err := Load(context.Background(), nil, &file)
	if err == nil || !strings.Contains(err.Error(), "unsupported content type") {
		t.Fatalf("expected unsupported content type error, got %v", err)
	}
}

func TestLoadRejectsOversizeImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		big := make([]byte, maxFetchBytes+10)
		_, _ = w.Write(big)
	}))
	defer srv.Close()

	file := domain.SourceFile{URI: srv.URL}
	err := Load(context.Background(), nil, &file)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestLoadHTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	file := domain.SourceFile{URI: srv.URL}
	err := Load(context.Background(), nil, &file)
	if err == nil || !strings.Contains(err.Error(), "unexpected status 404") {
		t.Fatalf("expected 404 error, got %v", err)
	}
}
