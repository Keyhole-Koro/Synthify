package jobstatus

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The tests in this file check firestore.rules on the Firestore emulator. They
// run only when FIRESTORE_EMULATOR_HOST is set and skip otherwise.

// emulatorProject loads firestore.rules into a fresh project on the emulator
// and returns a client for it.
func emulatorProject(t *testing.T) *rulesClient {
	t.Helper()
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	rules, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "firestore.rules"))
	if err != nil {
		t.Fatalf("read firestore.rules: %v", err)
	}
	c := &rulesClient{t: t, base: "http://" + host, project: fmt.Sprintf("demo-rules-%d", time.Now().UnixNano())}
	body, _ := json.Marshal(map[string]any{"rules": map[string]any{"files": []map[string]string{{"name": "firestore.rules", "content": string(rules)}}}})
	if code, resp := c.do(http.MethodPut, c.base+"/emulator/v1/projects/"+c.project+":securityRules", "", body); code != http.StatusOK {
		t.Fatalf("load rules: %d %s", code, resp)
	}
	return c
}

type rulesClient struct {
	t       *testing.T
	base    string
	project string
}

// adminToken bypasses the rules on the emulator, like the Admin SDK in the
// API and the worker.
const adminToken = "owner"

// userToken returns an unsigned ID token for uid, which the emulator accepts.
func (c *rulesClient) userToken(uid string) string {
	enc := base64.RawURLEncoding.EncodeToString
	header, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	now := time.Now().Unix()
	claims, _ := json.Marshal(map[string]any{
		"iss": "https://securetoken.google.com/" + c.project, "aud": c.project,
		"sub": uid, "user_id": uid, "iat": now, "auth_time": now, "exp": now + 3600,
		"email": uid + "@example.com", "email_verified": true,
		"firebase": map[string]any{"sign_in_provider": "google.com", "identities": map[string]any{}},
	})
	return enc(header) + "." + enc(claims) + "."
}

func (c *rulesClient) docURL(path string) string {
	return c.base + "/v1/projects/" + c.project + "/databases/(default)/documents/" + path
}

func (c *rulesClient) do(method, url, token string, body []byte) (int, string) {
	c.t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (c *rulesClient) write(path, token string, fields map[string]string) int {
	c.t.Helper()
	doc := map[string]any{}
	for k, v := range fields {
		doc[k] = map[string]string{"stringValue": v}
	}
	body, _ := json.Marshal(map[string]any{"fields": doc})
	code, _ := c.do(http.MethodPatch, c.docURL(path), token, body)
	return code
}

func (c *rulesClient) mustWrite(path string, fields map[string]string) {
	c.t.Helper()
	if code := c.write(path, adminToken, fields); code != http.StatusOK {
		c.t.Fatalf("seed %s: %d", path, code)
	}
}

func (c *rulesClient) get(path, token string) int {
	c.t.Helper()
	code, _ := c.do(http.MethodGet, c.docURL(path), token, nil)
	return code
}

// listJobs runs the query the web app runs (useWorkspaceJobStatuses).
func (c *rulesClient) listJobs(workspaceID, token string) int {
	c.t.Helper()
	body, _ := json.Marshal(map[string]any{"structuredQuery": map[string]any{
		"from":    []map[string]string{{"collectionId": "jobs"}},
		"orderBy": []map[string]any{{"field": map[string]string{"fieldPath": "updatedAt"}, "direction": "DESCENDING"}},
		"limit":   6,
	}})
	code, _ := c.do(http.MethodPost, c.docURL("workspaces/"+workspaceID)+":runQuery", token, body)
	return code
}

func (c *rulesClient) remove(path string) {
	c.t.Helper()
	if code, resp := c.do(http.MethodDelete, c.docURL(path), adminToken, nil); code != http.StatusOK {
		c.t.Fatalf("delete %s: %d %s", path, code, resp)
	}
}

func jobFields(workspaceID, jobID string) map[string]string {
	return map[string]string{
		"jobId": jobID, "workspaceId": workspaceID, "documentId": "doc-" + jobID,
		"status": "running", "suggestedWorkspaceName": "secret topic of " + workspaceID,
		"updatedAt": "2026-11-20T00:00:00Z",
	}
}

func TestFirestoreRules_JobStatusIsReadOnlyForTheWorkspaceReaders(t *testing.T) {
	c := emulatorProject(t)
	c.mustWrite("workspaces/ws-a/jobs/job-a", jobFields("ws-a", "job-a"))
	c.mustWrite("workspaces/ws-b/jobs/job-b", jobFields("ws-b", "job-b"))
	c.mustWrite("workspaces/ws-a/readers/alice", map[string]string{"userId": "alice"})
	c.mustWrite("workspaces/ws-b/readers/bob", map[string]string{"userId": "bob"})
	alice, bob, mallory := c.userToken("alice"), c.userToken("bob"), c.userToken("mallory")

	steps := []struct {
		name string
		got  func() int
		want int
	}{
		{"a reader gets a job of the workspace", func() int { return c.get("workspaces/ws-a/jobs/job-a", alice) }, http.StatusOK},
		{"a reader lists the workspace's jobs", func() int { return c.listJobs("ws-a", alice) }, http.StatusOK},
		{"a signed-in non-reader cannot get a job", func() int { return c.get("workspaces/ws-a/jobs/job-a", mallory) }, http.StatusForbidden},
		{"a signed-in non-reader cannot list the jobs", func() int { return c.listJobs("ws-a", mallory) }, http.StatusForbidden},
		{"a reader of another workspace cannot get a job", func() int { return c.get("workspaces/ws-a/jobs/job-a", bob) }, http.StatusForbidden},
		{"a reader of another workspace cannot list the jobs", func() int { return c.listJobs("ws-a", bob) }, http.StatusForbidden},
		{"an anonymous caller cannot get a job", func() int { return c.get("workspaces/ws-a/jobs/job-a", "") }, http.StatusForbidden},
		{"an anonymous caller cannot list the jobs", func() int { return c.listJobs("ws-a", "") }, http.StatusForbidden},
		{"a reader cannot write a job", func() int { return c.write("workspaces/ws-a/jobs/job-a", alice, jobFields("ws-a", "job-a")) }, http.StatusForbidden},
		{"nobody reads the reader list", func() int { return c.get("workspaces/ws-a/readers/alice", alice) }, http.StatusForbidden},
		{"nobody grants themselves", func() int {
			return c.write("workspaces/ws-a/readers/mallory", mallory, map[string]string{"userId": "mallory"})
		}, http.StatusForbidden},
	}
	for _, s := range steps {
		if got := s.got(); got != s.want {
			t.Errorf("%s: status %d, want %d", s.name, got, s.want)
		}
	}

	// Removing the reader document takes the access away.
	c.remove("workspaces/ws-a/readers/alice")
	if got := c.get("workspaces/ws-a/jobs/job-a", alice); got != http.StatusForbidden {
		t.Errorf("a removed reader gets a job: status %d, want %d", got, http.StatusForbidden)
	}
	if got := c.listJobs("ws-a", alice); got != http.StatusForbidden {
		t.Errorf("a removed reader lists the jobs: status %d, want %d", got, http.StatusForbidden)
	}
}
