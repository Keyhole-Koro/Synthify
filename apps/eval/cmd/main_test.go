package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/synthify/backend/apps/eval/runner"
)

func TestWriteReportFileCreatesParentDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "result.json")
	if err := writeReportFile(path, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("writeReportFile returned error: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(b) != `{"ok":true}` {
		t.Fatalf("unexpected output file content: %s", b)
	}
}

func TestAllPassed(t *testing.T) {
	if !allPassed(nil) {
		t.Fatal("empty result set should pass")
	}
	if !allPassed([]runner.Result{{Passed: true}, {Passed: true}}) {
		t.Fatal("all true results should pass")
	}
	if allPassed([]runner.Result{{Passed: true}, {Passed: false}}) {
		t.Fatal("any false result should fail")
	}
}

func TestResolveGCSOutputURI(t *testing.T) {
	t.Setenv("EVAL_OUTPUT_GCS_URI", "gs://env-bucket/eval")
	if got := resolveGCSOutputURI(" gs://flag-bucket/eval "); got != "gs://flag-bucket/eval" {
		t.Fatalf("flag should win, got %q", got)
	}
	if got := resolveGCSOutputURI(""); got != "gs://env-bucket/eval" {
		t.Fatalf("env fallback mismatch: %q", got)
	}
	t.Setenv("EVAL_OUTPUT_GCS_URI", "")
	if got := resolveGCSOutputURI(""); got != "" {
		t.Fatalf("empty gcs output should stay empty, got %q", got)
	}
}
