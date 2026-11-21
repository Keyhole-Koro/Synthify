package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	gcs "cloud.google.com/go/storage"
	"github.com/synthify/backend/apps/api/internal/repository"
	sharedstorage "github.com/synthify/backend/internal/platform/storage"
	"google.golang.org/api/iterator"
)

// NewObjectPrefixDeleter picks the Cloud Storage or fake-gcs implementation the
// same way NewDocumentObjectStore does.
func NewObjectPrefixDeleter(baseURL, bucket string) repository.ObjectPrefixDeleter {
	if usesGoogleCloudStorage(baseURL) {
		return NewGCSObjectPrefixDeleter(bucket, nil)
	}
	return NewFakeGCSObjectPrefixDeleter(baseURL, bucket, nil)
}

// GCSObjectPrefixDeleter deletes objects in Cloud Storage. The uploads bucket
// keeps noncurrent versions (terraform/modules/gcs_bucket), so a plain delete
// would only hide an object; every generation is listed and deleted instead.
type GCSObjectPrefixDeleter struct {
	bucket string
	client *gcs.Client
}

func NewGCSObjectPrefixDeleter(bucket string, client *gcs.Client) *GCSObjectPrefixDeleter {
	return &GCSObjectPrefixDeleter{bucket: bucket, client: client}
}

func (d *GCSObjectPrefixDeleter) DeleteObjectsWithPrefix(ctx context.Context, prefix string) (int, error) {
	if err := sharedstorage.ValidateDeletionPrefix(prefix); err != nil {
		return 0, err
	}
	client, err := d.clientOrDefault(ctx)
	if err != nil {
		return 0, err
	}
	bucket := client.Bucket(d.bucket)
	query := &gcs.Query{Prefix: prefix, Versions: true}
	if err := query.SetAttrSelection([]string{"Name", "Generation"}); err != nil {
		return 0, err
	}
	it := bucket.Objects(ctx, query)
	deleted := 0
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return deleted, nil
		}
		if err != nil {
			return deleted, fmt.Errorf("list objects under %q: %w", prefix, err)
		}
		err = bucket.Object(attrs.Name).Generation(attrs.Generation).Delete(ctx)
		if err != nil && !errors.Is(err, gcs.ErrObjectNotExist) {
			return deleted, fmt.Errorf("delete object %q generation %d: %w", attrs.Name, attrs.Generation, err)
		}
		deleted++
	}
}

func (d *GCSObjectPrefixDeleter) clientOrDefault(ctx context.Context) (*gcs.Client, error) {
	if d.client != nil {
		return d.client, nil
	}
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	d.client = client
	return d.client, nil
}

// FakeGCSObjectPrefixDeleter deletes objects through the fake-gcs JSON API used
// by local compose and integration tests.
type FakeGCSObjectPrefixDeleter struct {
	baseURL string
	bucket  string
	client  *http.Client
}

func NewFakeGCSObjectPrefixDeleter(baseURL, bucket string, client *http.Client) *FakeGCSObjectPrefixDeleter {
	if client == nil {
		client = http.DefaultClient
	}
	return &FakeGCSObjectPrefixDeleter{baseURL: strings.TrimRight(baseURL, "/"), bucket: bucket, client: client}
}

type fakeGCSObjectList struct {
	Items []struct {
		Name       string `json:"name"`
		Generation string `json:"generation"`
	} `json:"items"`
	NextPageToken string `json:"nextPageToken"`
}

func (d *FakeGCSObjectPrefixDeleter) DeleteObjectsWithPrefix(ctx context.Context, prefix string) (int, error) {
	if err := sharedstorage.ValidateDeletionPrefix(prefix); err != nil {
		return 0, err
	}
	deleted := 0
	pageToken := ""
	for {
		page, err := d.list(ctx, prefix, pageToken)
		if err != nil {
			return deleted, err
		}
		for _, item := range page.Items {
			if err := d.delete(ctx, item.Name, item.Generation); err != nil {
				return deleted, err
			}
			deleted++
		}
		if page.NextPageToken == "" {
			return deleted, nil
		}
		pageToken = page.NextPageToken
	}
}

func (d *FakeGCSObjectPrefixDeleter) list(ctx context.Context, prefix, pageToken string) (*fakeGCSObjectList, error) {
	listURL := sharedstorage.BuildObjectListURL(d.baseURL, d.bucket, prefix) + "&versions=true"
	if pageToken != "" {
		listURL += "&pageToken=" + url.QueryEscape(pageToken)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list objects under %q: %w", prefix, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list objects under %q failed status=%d: %s", prefix, resp.StatusCode, string(body))
	}
	var page fakeGCSObjectList
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("decode object list under %q: %w", prefix, err)
	}
	return &page, nil
}

func (d *FakeGCSObjectPrefixDeleter) delete(ctx context.Context, name, generation string) error {
	deleteURL := fmt.Sprintf("%s/storage/v1/b/%s/o/%s", d.baseURL, d.bucket, url.PathEscape(name))
	if generation != "" {
		deleteURL += "?generation=" + url.QueryEscape(generation)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete object %q: %w", name, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return nil
	}
	return fmt.Errorf("delete object %q failed status=%d: %s", name, resp.StatusCode, string(body))
}
