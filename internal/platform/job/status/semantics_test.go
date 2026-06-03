package jobstatus

import "testing"

func TestFirestoreJobStatusSemantics(t *testing.T) {
	if !IsInFlight(FirestoreJobStatusStateQueued) {
		t.Fatalf("queued should be in-flight")
	}
	if !IsInFlight(FirestoreJobStatusStateRunning) {
		t.Fatalf("running should be in-flight")
	}
	if IsInFlight(FirestoreJobStatusStateSucceeded) {
		t.Fatalf("succeeded should not be in-flight")
	}
	if !IsTerminal(FirestoreJobStatusStateSucceeded) {
		t.Fatalf("succeeded should be terminal")
	}
	if !IsTerminal(FirestoreJobStatusStateFailed) {
		t.Fatalf("failed should be terminal")
	}
}

func TestHasCreatedDocumentRoot(t *testing.T) {
	rootID := "doc_root_1"
	if !HasCreatedDocumentRoot(FirestoreJobStatus{
		Status:                    FirestoreJobStatusStateSucceeded,
		CreatedDocumentRootItemID: &rootID,
	}) {
		t.Fatalf("succeeded job with document root id should have created document root")
	}
	if HasCreatedDocumentRoot(FirestoreJobStatus{
		Status:                    FirestoreJobStatusStateFailed,
		CreatedDocumentRootItemID: &rootID,
	}) {
		t.Fatalf("failed job should not have created document root semantics")
	}
	if HasCreatedDocumentRoot(FirestoreJobStatus{
		Status: FirestoreJobStatusStateSucceeded,
	}) {
		t.Fatalf("succeeded job without document root id should not have created document root")
	}
}
