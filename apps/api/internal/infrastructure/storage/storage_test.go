package storage

import (
	"context"
	"testing"
)

func TestStorageFactorySelectsGCSOrFakeImplementation(t *testing.T) {
	if got := NewObjectMetadataFetcher("https://storage.googleapis.com", "bucket"); got == nil {
		t.Fatal("NewObjectMetadataFetcher() returned nil")
	} else if _, ok := got.(*GCSObjectMetadataFetcher); !ok {
		t.Fatalf("GCS factory returned %T", got)
	}
	if got := NewObjectMetadataFetcher("http://fake-gcs:4443", "bucket"); got == nil {
		t.Fatal("fake metadata factory returned nil")
	} else if _, ok := got.(*FakeGCSObjectMetadataFetcher); !ok {
		t.Fatalf("fake factory returned %T", got)
	}
	if got := NewDocumentObjectStore("http://fake-gcs:4443", "bucket"); got == nil {
		t.Fatal("fake object store factory returned nil")
	} else if _, ok := got.(*FakeGCSDocumentObjectStore); !ok {
		t.Fatalf("fake object store returned %T", got)
	}
}

func TestParseObjectSize(t *testing.T) {
	if got, err := parseObjectSize("42"); err != nil || got != 42 {
		t.Fatalf("string size = %d, error = %v", got, err)
	}
	if got, err := parseObjectSize(float64(7)); err != nil || got != 7 {
		t.Fatalf("number size = %d, error = %v", got, err)
	}
	if _, err := parseObjectSize(true); err == nil {
		t.Fatal("unsupported size type accepted")
	}
}

func TestFakeUploadURLIssuer(t *testing.T) {
	issuer := NewFakeGCSDocumentUploadURLIssuer("http://fake-gcs:4443", "uploads")
	got, err := issuer.IssueDocumentUploadURL(context.Background(), "ws_1", "doc_1/source/original.pdf", "application/pdf", 12)
	if err != nil {
		t.Fatalf("IssueDocumentUploadURL() error = %v", err)
	}
	if got.Method != "POST" || got.ContentType != "application/pdf" {
		t.Fatalf("target = %+v", got)
	}
	if got.URL == "" {
		t.Fatal("upload URL is empty")
	}
}

func TestUsesGoogleCloudStorage(t *testing.T) {
	if !usesGoogleCloudStorage("https://storage.googleapis.com") {
		t.Fatal("GCS host not detected")
	}
	if usesGoogleCloudStorage("http://storage.googleapis.com.local") {
		t.Fatal("lookalike host detected as GCS")
	}
}
