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
	"strings"

	gcs "cloud.google.com/go/storage"
	"github.com/synthify/backend/apps/api/internal/domain"
	sharedstorage "github.com/synthify/backend/internal/platform/storage"
	"google.golang.org/api/iterator"
)

type objectMetadataFetcher interface {
	GetObjectMetadata(ctx context.Context, workspaceID, documentID string) (*domain.ObjectMetadata, error)
}

type objectDeleter interface {
	DeleteDocumentObject(ctx context.Context, workspaceID, documentID string) error
}

func NewObjectMetadataFetcher(baseURL, bucket string) objectMetadataFetcher {
	if usesGoogleCloudStorage(baseURL) {
		return NewGCSObjectMetadataFetcher(bucket, nil)
	}
	return NewFakeGCSObjectMetadataFetcher(baseURL, bucket, nil)
}

func NewDocumentObjectStore(baseURL, bucket string) objectDeleter {
	if usesGoogleCloudStorage(baseURL) {
		return NewGCSDocumentObjectStore(bucket, nil)
	}
	return NewFakeGCSDocumentObjectStore(baseURL, bucket, nil)
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
	// The original lives at {ws}/{doc}/source/original.{ext}; list the source/
	// prefix and take the single object the uploader placed there.
	prefix := workspaceID + "/" + sharedstorage.SourcePrefix(documentID)
	listURL := sharedstorage.BuildObjectListURL(f.baseURL, f.bucket, prefix)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("object list request failed status=%d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		Items []struct {
			Size        any    `json:"size"`
			ContentType string `json:"contentType"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if len(raw.Items) == 0 {
		return nil, domain.ErrUploadNotConfirmed
	}
	size, err := parseObjectSize(raw.Items[0].Size)
	if err != nil {
		return nil, err
	}
	return &domain.ObjectMetadata{
		Size:        size,
		ContentType: raw.Items[0].ContentType,
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
	// The uploaded original lives at {ws}/{doc}/source/original.{ext}; we don't
	// know the extension here, so list the source/ prefix and take the single
	// object the uploader placed there.
	prefix := workspaceID + "/" + sharedstorage.SourcePrefix(documentID)
	it := client.Bucket(f.bucket).Objects(ctx, &gcs.Query{Prefix: prefix})
	attrs, err := it.Next()
	if err == iterator.Done {
		return nil, domain.ErrUploadNotConfirmed
	}
	if err != nil {
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

type FakeGCSDocumentObjectStore struct {
	baseURL string
	bucket  string
	client  *http.Client
}

func NewFakeGCSDocumentObjectStore(baseURL, bucket string, client *http.Client) *FakeGCSDocumentObjectStore {
	if client == nil {
		client = http.DefaultClient
	}
	return &FakeGCSDocumentObjectStore{baseURL: baseURL, bucket: bucket, client: client}
}

func (f *FakeGCSDocumentObjectStore) DeleteDocumentObject(ctx context.Context, workspaceID, documentID string) error {
	// The document is a subtree now ({doc}/source/ + {doc}/extracted/); list and
	// delete every object under {ws}/{doc}/.
	prefix := workspaceID + "/" + sharedstorage.DocumentPrefix(documentID)
	listURL := sharedstorage.BuildObjectListURL(f.baseURL, f.bucket, prefix)
	listReq, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return err
	}
	listResp, err := f.client.Do(listReq)
	if err != nil {
		return err
	}
	listBody, _ := io.ReadAll(io.LimitReader(listResp.Body, 1<<20))
	listResp.Body.Close()
	if listResp.StatusCode < 200 || listResp.StatusCode >= 300 {
		return fmt.Errorf("list objects for delete failed status=%d: %s", listResp.StatusCode, string(listBody))
	}
	var raw struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listBody, &raw); err != nil {
		return err
	}
	root := strings.TrimRight(f.baseURL, "/")
	for _, item := range raw.Items {
		delURL := fmt.Sprintf("%s/storage/v1/b/%s/o/%s", root, f.bucket, url.PathEscape(item.Name))
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, delURL, nil)
		if err != nil {
			return err
		}
		resp, err := f.client.Do(req)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("delete object %q failed status=%d: %s", item.Name, resp.StatusCode, string(body))
		}
	}
	return nil
}

type GCSDocumentObjectStore struct {
	bucket string
	client *gcs.Client
}

func NewGCSDocumentObjectStore(bucket string, client *gcs.Client) *GCSDocumentObjectStore {
	return &GCSDocumentObjectStore{bucket: bucket, client: client}
}

func (s *GCSDocumentObjectStore) DeleteDocumentObject(ctx context.Context, workspaceID, documentID string) error {
	client, err := s.clientOrDefault(ctx)
	if err != nil {
		return err
	}
	// Delete the whole document subtree ({doc}/source/ + {doc}/extracted/), not a
	// single object, since the document is now a directory.
	bucket := client.Bucket(s.bucket)
	prefix := workspaceID + "/" + sharedstorage.DocumentPrefix(documentID)
	it := bucket.Objects(ctx, &gcs.Query{Prefix: prefix})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("list document objects for delete: %w", err)
		}
		if err := bucket.Object(attrs.Name).Delete(ctx); err != nil && !errors.Is(err, gcs.ErrObjectNotExist) {
			return fmt.Errorf("delete document object %q: %w", attrs.Name, err)
		}
	}
	return nil
}

func (s *GCSDocumentObjectStore) clientOrDefault(ctx context.Context) (*gcs.Client, error) {
	if s.client != nil {
		return s.client, nil
	}
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	s.client = client
	return s.client, nil
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
