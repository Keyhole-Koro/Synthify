package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	gcs "cloud.google.com/go/storage"
	"github.com/synthify/backend/apps/api/internal/domain"
	sharedstorage "github.com/synthify/backend/internal/platform/storage"
)

type objectMetadataFetcher interface {
	GetObjectMetadata(ctx context.Context, workspaceID, documentID string) (*domain.ObjectMetadata, error)
}

func NewObjectMetadataFetcher(baseURL, bucket string) objectMetadataFetcher {
	if usesGoogleCloudStorage(baseURL) {
		return NewGCSObjectMetadataFetcher(bucket, nil)
	}
	return NewFakeGCSObjectMetadataFetcher(baseURL, bucket, nil)
}

// FakeGCSObjectMetadataFetcher reads metadata through the fake-gcs JSON API
// used by local compose and integration tests. Real GCS uses the authenticated
// SDK-backed fetcher below.
type FakeGCSObjectMetadataFetcher struct {
	baseURL string
	bucket  string
	client  *http.Client
}

func NewFakeGCSObjectMetadataFetcher(baseURL, bucket string, client *http.Client) *FakeGCSObjectMetadataFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	return &FakeGCSObjectMetadataFetcher{
		baseURL: baseURL,
		bucket:  bucket,
		client:  client,
	}
}

func (f *FakeGCSObjectMetadataFetcher) GetObjectMetadata(ctx context.Context, workspaceID, documentID string) (*domain.ObjectMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sharedstorage.BuildDocumentObjectMetadataURL(f.baseURL, f.bucket, workspaceID, documentID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrUploadNotConfirmed
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("object metadata request failed status=%d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		Size        any    `json:"size"`
		ContentType string `json:"contentType"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	size, err := parseObjectSize(raw.Size)
	if err != nil {
		return nil, err
	}
	return &domain.ObjectMetadata{
		Size:        size,
		ContentType: raw.ContentType,
	}, nil
}

type GCSObjectMetadataFetcher struct {
	bucket string
	client *gcs.Client
}

func NewGCSObjectMetadataFetcher(bucket string, client *gcs.Client) *GCSObjectMetadataFetcher {
	return &GCSObjectMetadataFetcher{bucket: bucket, client: client}
}

func (f *GCSObjectMetadataFetcher) GetObjectMetadata(ctx context.Context, workspaceID, documentID string) (*domain.ObjectMetadata, error) {
	client, err := f.clientOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	attrs, err := client.Bucket(f.bucket).Object(workspaceID + "/" + documentID).Attrs(ctx)
	if err != nil {
		if errors.Is(err, gcs.ErrObjectNotExist) {
			return nil, domain.ErrUploadNotConfirmed
		}
		return nil, fmt.Errorf("get object metadata: %w", err)
	}
	return &domain.ObjectMetadata{
		Size:        attrs.Size,
		ContentType: attrs.ContentType,
	}, nil
}

func (f *GCSObjectMetadataFetcher) clientOrDefault(ctx context.Context) (*gcs.Client, error) {
	if f.client != nil {
		return f.client, nil
	}
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	f.client = client
	return f.client, nil
}

func usesGoogleCloudStorage(rawBaseURL string) bool {
	parsed, err := url.Parse(rawBaseURL)
	if err != nil {
		return false
	}
	return parsed.Host == "storage.googleapis.com"
}

func parseObjectSize(value any) (int64, error) {
	switch typed := value.(type) {
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case float64:
		return int64(typed), nil
	default:
		return 0, errors.New("object metadata size is missing")
	}
}
