package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	gcs "cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

type storedObject struct {
	name       string
	generation int64
}

// fakeJSONAPI serves the slice of the Cloud Storage JSON API that both
// deleters use: object listing (paged, two per page, optionally with
// noncurrent versions) and object deletion by name and generation.
type fakeJSONAPI struct {
	mu          sync.Mutex
	bucket      string
	objects     []storedObject
	deleted     []string // "name#generation"
	listQueries []string
	failDelete  string // object name whose deletion answers 403, as without IAM delete permission
	missing     string // object name whose deletion answers 404
}

func (f *fakeJSONAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	listPath := "/storage/v1/b/" + f.bucket + "/o"
	switch {
	case r.Method == http.MethodGet && r.URL.Path == listPath:
		f.list(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, listPath+"/"):
		name := strings.TrimPrefix(r.URL.Path, listPath+"/")
		generation := r.URL.Query().Get("generation")
		switch name {
		case f.failDelete:
			// 403 rather than 5xx: the client retries 5xx on idempotent calls.
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		case f.missing:
			http.Error(w, "gone", http.StatusNotFound)
			return
		}
		f.deleted = append(f.deleted, name+"#"+generation)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unexpected "+r.Method+" "+r.URL.String(), http.StatusBadRequest)
	}
}

func (f *fakeJSONAPI) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	f.listQueries = append(f.listQueries, query.Get("prefix")+" versions="+query.Get("versions"))
	var matching []storedObject
	for _, object := range f.objects {
		if strings.HasPrefix(object.name, query.Get("prefix")) {
			matching = append(matching, object)
		}
	}
	start, _ := strconv.Atoi(query.Get("pageToken"))
	end := min(start+2, len(matching))
	page := map[string]any{"kind": "storage#objects"}
	items := []map[string]any{}
	for _, object := range matching[start:end] {
		items = append(items, map[string]any{
			"kind":       "storage#object",
			"bucket":     f.bucket,
			"name":       object.name,
			"generation": strconv.FormatInt(object.generation, 10),
		})
	}
	page["items"] = items
	if end < len(matching) {
		page["nextPageToken"] = strconv.Itoa(end)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(page)
}

func (f *fakeJSONAPI) deletedSorted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]string(nil), f.deleted...)
	sort.Strings(out)
	return out
}

func newFakeJSONAPI(t *testing.T) (*fakeJSONAPI, *httptest.Server) {
	t.Helper()
	api := &fakeJSONAPI{
		bucket: "uploads",
		objects: []storedObject{
			{"ws1/doc1/source/original.pdf", 1},
			{"ws1/doc1/source/original.pdf", 2}, // noncurrent + live generation
			{"ws1/doc1/extracted/page 1.txt", 3},
			{"ws1/doc10/source/original.pdf", 4}, // shares the "doc1" characters, not the prefix
			{"ws2/doc1/source/original.pdf", 5},
		},
	}
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	return api, server
}

func newTestGCSDeleter(t *testing.T, server *httptest.Server) *GCSObjectPrefixDeleter {
	t.Helper()
	client, err := gcs.NewClient(context.Background(),
		option.WithEndpoint(server.URL+"/storage/v1/"),
		option.WithoutAuthentication(),
		option.WithHTTPClient(server.Client()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return NewGCSObjectPrefixDeleter("uploads", client)
}

func TestGCSObjectPrefixDeleter_DeletesEveryGenerationUnderThePrefixOnly(t *testing.T) {
	api, server := newFakeJSONAPI(t)
	deleter := newTestGCSDeleter(t, server)

	deleted, err := deleter.DeleteObjectsWithPrefix(context.Background(), "ws1/doc1/")

	require.NoError(t, err)
	assert.Equal(t, 3, deleted)
	assert.Equal(t, []string{
		"ws1/doc1/extracted/page 1.txt#3",
		"ws1/doc1/source/original.pdf#1",
		"ws1/doc1/source/original.pdf#2",
	}, api.deletedSorted())
	require.NotEmpty(t, api.listQueries)
	assert.Equal(t, "ws1/doc1/ versions=true", api.listQueries[0])
}

func TestGCSObjectPrefixDeleter_StopsOnDeleteFailure(t *testing.T) {
	api, server := newFakeJSONAPI(t)
	api.failDelete = "ws1/doc1/source/original.pdf"
	deleter := newTestGCSDeleter(t, server)

	_, err := deleter.DeleteObjectsWithPrefix(context.Background(), "ws1/doc1/")

	require.Error(t, err)
}

func TestGCSObjectPrefixDeleter_TreatsMissingObjectsAsDeleted(t *testing.T) {
	api, server := newFakeJSONAPI(t)
	api.missing = "ws1/doc1/extracted/page 1.txt"
	deleter := newTestGCSDeleter(t, server)

	_, err := deleter.DeleteObjectsWithPrefix(context.Background(), "ws1/doc1/")

	require.NoError(t, err)
}

func TestFakeGCSObjectPrefixDeleter_DeletesAcrossPages(t *testing.T) {
	api, server := newFakeJSONAPI(t)
	deleter := NewFakeGCSObjectPrefixDeleter(server.URL, "uploads", server.Client())

	deleted, err := deleter.DeleteObjectsWithPrefix(context.Background(), "ws1/")

	require.NoError(t, err)
	assert.Equal(t, 4, deleted)
	assert.Equal(t, []string{
		"ws1/doc1/extracted/page 1.txt#3",
		"ws1/doc1/source/original.pdf#1",
		"ws1/doc1/source/original.pdf#2",
		"ws1/doc10/source/original.pdf#4",
	}, api.deletedSorted())
	assert.Equal(t, []string{"ws1/ versions=true", "ws1/ versions=true"}, api.listQueries)
}

func TestFakeGCSObjectPrefixDeleter_ReportsDeleteFailure(t *testing.T) {
	api, server := newFakeJSONAPI(t)
	api.failDelete = "ws1/doc1/source/original.pdf"
	deleter := NewFakeGCSObjectPrefixDeleter(server.URL, "uploads", server.Client())

	_, err := deleter.DeleteObjectsWithPrefix(context.Background(), "ws1/doc1/")

	require.Error(t, err)
}

// A prefix that could match other owners' objects is refused before any
// request reaches the bucket.
func TestObjectPrefixDeleters_RefuseUnsafePrefixes(t *testing.T) {
	api, server := newFakeJSONAPI(t)
	deleters := map[string]interface {
		DeleteObjectsWithPrefix(context.Context, string) (int, error)
	}{
		"gcs":  newTestGCSDeleter(t, server),
		"fake": NewFakeGCSObjectPrefixDeleter(server.URL, "uploads", server.Client()),
	}
	for name, deleter := range deleters {
		for _, prefix := range []string{"", "/", "ws1", ".cache/v1/", ".checkpoints/"} {
			t.Run(fmt.Sprintf("%s %q", name, prefix), func(t *testing.T) {
				_, err := deleter.DeleteObjectsWithPrefix(context.Background(), prefix)
				require.Error(t, err)
			})
		}
	}
	assert.Empty(t, api.listQueries)
	assert.Empty(t, api.deletedSorted())
}
