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

type GCSConfig struct {
	PrefixURI string
	Now       func() time.Time
	Rand      io.Reader
}

type UploadResult struct {
	URI string
}

type ObjectWriter interface {
	WriteObject(ctx context.Context, bucket, object, contentType string, data []byte) error
}

type GCSWriter struct {
	Client *gcs.Client
}

func UploadGCS(ctx context.Context, cfg GCSConfig, report []byte) (UploadResult, error) {
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return UploadResult{}, fmt.Errorf("create gcs client: %w", err)
	}
	defer client.Close()
	return UploadGCSWithWriter(ctx, cfg, report, GCSWriter{Client: client})
}

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
	datePath := path.Join(t.Format("2006"), t.Format("01"), t.Format("02"), filename)
	prefix = normalizePrefix(prefix)
	if prefix == "" {
		return datePath, nil
	}
	return path.Join(prefix, datePath), nil
}

func normalizePrefix(prefix string) string {
	return strings.Trim(path.Clean("/"+strings.TrimSpace(prefix)), "/")
}

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
