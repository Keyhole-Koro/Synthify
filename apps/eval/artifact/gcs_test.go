package artifact

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeWriter struct {
	bucket      string
	object      string
	contentType string
	data        []byte
	err         error
}

func (f *fakeWriter) WriteObject(_ context.Context, bucket, object, contentType string, data []byte) error {
	if f.err != nil {
		return f.err
	}
	f.bucket = bucket
	f.object = object
	f.contentType = contentType
	f.data = append([]byte(nil), data...)
	return nil
}

func TestParseGCSPrefix(t *testing.T) {
	bucket, prefix, err := ParseGCSPrefix("gs://bucket/eval/stage/runs/")
	if err != nil {
		t.Fatalf("ParseGCSPrefix returned error: %v", err)
	}
	if bucket != "bucket" || prefix != "eval/stage/runs" {
		t.Fatalf("unexpected parse result: bucket=%q prefix=%q", bucket, prefix)
	}
}

func TestParseGCSPrefixRejectsInvalidURI(t *testing.T) {
	for _, raw := range []string{"", "http://bucket/path", "gs:///path", "gs://bucket/path?q=1"} {
		if _, _, err := ParseGCSPrefix(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestBuildObjectName(t *testing.T) {
	object, err := BuildObjectName("/eval/stage/runs/", GCSConfig{
		Now:  func() time.Time { return time.Date(2026, 5, 17, 4, 20, 0, 0, time.FixedZone("JST", 9*60*60)) },
		Rand: bytes.NewReader([]byte{0xa1, 0xb2, 0xc3}),
	})
	if err != nil {
		t.Fatalf("BuildObjectName returned error: %v", err)
	}
	want := "eval/stage/runs/2026/05/16/20260516T192000Z-a1b2c3.json"
	if object != want {
		t.Fatalf("unexpected object name: got %q want %q", object, want)
	}
}

func TestUploadGCSWithWriter(t *testing.T) {
	fw := &fakeWriter{}
	res, err := UploadGCSWithWriter(context.Background(), GCSConfig{
		PrefixURI: "gs://bucket/eval/stage/runs",
		Now:       func() time.Time { return time.Date(2026, 5, 17, 4, 20, 0, 0, time.UTC) },
		Rand:      bytes.NewReader([]byte{0x01, 0x02, 0x03}),
	}, []byte(`{"ok":true}`), fw)
	if err != nil {
		t.Fatalf("UploadGCSWithWriter returned error: %v", err)
	}
	if res.URI != "gs://bucket/eval/stage/runs/2026/05/17/20260517T042000Z-010203.json" {
		t.Fatalf("unexpected uri: %q", res.URI)
	}
	if fw.bucket != "bucket" || fw.object != "eval/stage/runs/2026/05/17/20260517T042000Z-010203.json" {
		t.Fatalf("unexpected write target: %#v", fw)
	}
	if fw.contentType != jsonContentType || string(fw.data) != `{"ok":true}` {
		t.Fatalf("unexpected write content: %#v", fw)
	}
}

func TestUploadGCSWithWriterPropagatesErrors(t *testing.T) {
	_, err := UploadGCSWithWriter(context.Background(), GCSConfig{
		PrefixURI: "gs://bucket/eval",
		Now:       func() time.Time { return time.Date(2026, 5, 17, 4, 20, 0, 0, time.UTC) },
		Rand:      bytes.NewReader([]byte{0x01, 0x02, 0x03}),
	}, []byte(`{}`), &fakeWriter{err: errors.New("boom")})
	if err == nil {
		t.Fatal("expected upload error")
	}
}
