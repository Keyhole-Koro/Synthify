// Package artifact provides utilities for uploading evaluation run reports
// and logs to cloud storage destinations such as Google Cloud Storage (GCS).
package artifact

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	gcs "cloud.google.com/go/storage"
)

const jsonContentType = "application/json"

// GCSConfig specifies upload destination settings, path segmentation, and clock/random sources.
type GCSConfig struct {
	// PrefixURI is the root GCS URI destination (e.g. "gs://bucket/prefix").
	PrefixURI string
	// PromptSource segments the object path so baseline and variant runs
	// are separable from the URI alone (e.g. "production",
	// "variant:concise-v1"). Empty falls back to "production".
	PromptSource string
	// Now returns the current timestamp, defaulting to time.Now if nil.
	Now          func() time.Time
	// Rand provides cryptographically secure entropy, defaulting to crypto/rand if nil.
	Rand         io.Reader
}
// UploadResult contains the metadata and canonical URI of an uploaded artifact.
type UploadResult struct {
	// URI is the canonical gs:// path to the uploaded artifact.
	URI string
}

// ObjectWriter abstracts writing raw bytes to an object store bucket.
type ObjectWriter interface {
	WriteObject(ctx context.Context, bucket, object, contentType string, data []byte) error
}

// GCSWriter implements ObjectWriter using the official Google Cloud Storage client.
type GCSWriter struct {
	Client *gcs.Client
}

// UploadGCS initializes a default GCS client and uploads the evaluation report bytes.
func UploadGCS(ctx context.Context, cfg GCSConfig, report []byte) (UploadResult, error) {
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return UploadResult{}, fmt.Errorf("create gcs client: %w", err)
	}
	defer client.Close()
	return UploadGCSWithWriter(ctx, cfg, report, GCSWriter{Client: client})
}

// UploadGCSWithWriter uploads report data to GCS using the specified ObjectWriter.
func UploadGCSWithWriter(ctx context.Context, cfg GCSConfig, report []byte, writer ObjectWriter) (UploadResult, error) {
	if writer == nil {
		return UploadResult{}, fmt.Errorf("gcs writer is required")
	}
	bucket, prefix, err := ParseGCSPrefix(cfg.PrefixURI)
	if err != nil {
		return UploadResult{}, err
	}
	object, err := BuildObjectName(prefix, cfg)
	if err != nil {
		return UploadResult{}, err
	}
	if err := writer.WriteObject(ctx, bucket, object, jsonContentType, report); err != nil {
		return UploadResult{}, fmt.Errorf("write gcs artifact: %w", err)
	}
	return UploadResult{URI: "gs://" + bucket + "/" + object}, nil
}

// WriteObject writes data to the specified GCS bucket and object path with precondition checks.
func (w GCSWriter) WriteObject(ctx context.Context, bucket, object, contentType string, data []byte) error {
	if w.Client == nil {
		return fmt.Errorf("gcs client is required")
	}
	obj := w.Client.Bucket(bucket).Object(object).If(gcs.Conditions{DoesNotExist: true})
	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

// ParseGCSPrefix parses and validates a GCS prefix URI (gs://bucket/prefix).
func ParseGCSPrefix(raw string) (bucket, prefix string, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", fmt.Errorf("gcs uri is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("parse gcs uri: %w", err)
	}
	if parsed.Scheme != "gs" {
		return "", "", fmt.Errorf("gcs uri must use gs:// scheme")
	}
	if parsed.Host == "" {
		return "", "", fmt.Errorf("gcs uri bucket is required")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", fmt.Errorf("gcs uri must not include query or fragment")
	}
	return parsed.Host, normalizePrefix(parsed.Path), nil
}

// BuildObjectName generates a timestamped and partitioned object path for an evaluation artifact.
func BuildObjectName(prefix string, cfg GCSConfig) (string, error) {
	now := time.Now
	if cfg.Now != nil {
		now = cfg.Now
	}
	t := now().UTC()
	suffix, err := randomSuffix(cfg.Rand)
	if err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%s-%s.json", t.Format("20060102T150405Z"), suffix)
	sourceSeg := promptSourceSegment(cfg.PromptSource)
	datePath := path.Join(sourceSeg, t.Format("2006"), t.Format("01"), t.Format("02"), filename)
	prefix = normalizePrefix(prefix)
	if prefix == "" {
		return datePath, nil
	}
	return path.Join(prefix, datePath), nil
}

// promptSourceSegment maps a report prompt_source ("production",
// "variant:concise-v1") to a single safe path segment. ":" is replaced so the
// variant name does not introduce an extra path level.
func promptSourceSegment(promptSource string) string {
	s := strings.TrimSpace(promptSource)
	if s == "" {
		return "production"
	}
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}

// normalizePrefix cleans and normalizes a storage prefix string without leading or trailing slashes.
func normalizePrefix(prefix string) string {
	return strings.Trim(path.Clean("/"+strings.TrimSpace(prefix)), "/")
}

// randomSuffix generates a 3-byte cryptographically random hex suffix.
func randomSuffix(reader io.Reader) (string, error) {
	if reader == nil {
		reader = rand.Reader
	}
	buf := make([]byte, 3)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", fmt.Errorf("generate run suffix: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
