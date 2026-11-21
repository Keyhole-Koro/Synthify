package jobstatus

import (
	"context"
	"net/http"
	"testing"

	"cloud.google.com/go/firestore"
)

// TestReaderAccess_DecidesWhoReadsJobStatus writes the reader documents with
// the Firestore client the API uses and checks, through firestore.rules on the
// emulator, who can then read the job status documents. It runs only when
// FIRESTORE_EMULATOR_HOST is set.
func TestReaderAccess_DecidesWhoReadsJobStatus(t *testing.T) {
	c := emulatorProject(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, c.project)
	if err != nil {
		t.Fatalf("firestore client: %v", err)
	}
	defer client.Close()
	access := NewFirestoreReaderAccess(client)

	c.mustWrite("workspaces/ws-a/jobs/job-a", jobFields("ws-a", "job-a"))
	c.mustWrite("workspaces/ws-b/jobs/job-b", jobFields("ws-b", "job-b"))
	alice, carol := c.userToken("alice"), c.userToken("carol")

	reads := func(token, workspaceID, jobID string) bool {
		t.Helper()
		get := c.get("workspaces/"+workspaceID+"/jobs/"+jobID, token)
		list := c.listJobs(workspaceID, token)
		if (get == http.StatusOK) != (list == http.StatusOK) {
			t.Fatalf("get %d and list %d disagree on %s", get, list, workspaceID)
		}
		return get == http.StatusOK
	}

	if reads(alice, "ws-a", "job-a") {
		t.Fatal("alice reads ws-a before any grant")
	}

	// A repeated workspace is written once.
	if err := access.Grant(ctx, "alice", "ws-a", "ws-b", "ws-a"); err != nil {
		t.Fatalf("grant alice: %v", err)
	}
	if err := access.Grant(ctx, "carol", "ws-a"); err != nil {
		t.Fatalf("grant carol: %v", err)
	}
	if !reads(alice, "ws-a", "job-a") || !reads(alice, "ws-b", "job-b") {
		t.Fatal("alice cannot read the workspaces she was granted")
	}
	if reads(carol, "ws-b", "job-b") {
		t.Fatal("carol reads ws-b, which she was not granted")
	}

	if err := access.Revoke(ctx, "alice", "ws-a"); err != nil {
		t.Fatalf("revoke alice: %v", err)
	}
	if reads(alice, "ws-a", "job-a") {
		t.Error("alice still reads ws-a after the revoke")
	}
	if !reads(alice, "ws-b", "job-b") {
		t.Error("revoking ws-a took ws-b away from alice")
	}
	if !reads(carol, "ws-a", "job-a") {
		t.Error("revoking alice took ws-a away from carol")
	}
	// Revoking what is not granted is not an error.
	if err := access.Revoke(ctx, "alice", "ws-a"); err != nil {
		t.Errorf("second revoke: %v", err)
	}

	if err := access.RevokeAll(ctx, "ws-a"); err != nil {
		t.Fatalf("revoke all of ws-a: %v", err)
	}
	if reads(carol, "ws-a", "job-a") {
		t.Error("carol still reads ws-a after it was revoked from everyone")
	}
	if !reads(alice, "ws-b", "job-b") {
		t.Error("revoking everyone from ws-a took ws-b away from alice")
	}
}

func TestReaderAccess_RejectsIDsThatAreNotOneDocument(t *testing.T) {
	// The IDs are checked before any request, so no Firestore is needed.
	access := NewFirestoreReaderAccess(nil)
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"empty user", access.Grant(ctx, "", "ws-a")},
		{"user with a slash", access.Grant(ctx, "alice/x", "ws-a")},
		{"workspace with a slash", access.Grant(ctx, "alice", "ws-a/readers")},
		{"dot-dot workspace", access.Revoke(ctx, "alice", "..")},
		{"empty workspace", access.RevokeAll(ctx, "")},
	} {
		if tc.err == nil {
			t.Errorf("%s: no error", tc.name)
		}
	}
}
