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

func TestChangedTree(t *testing.T) {
	changed := true
	if !ChangedTree(FirestoreJobStatus{
		Status:      FirestoreJobStatusStateSucceeded,
		TreeChanged: &changed,
	}) {
		t.Fatalf("succeeded job with treeChanged should report a tree change")
	}
	if ChangedTree(FirestoreJobStatus{
		Status:      FirestoreJobStatusStateFailed,
		TreeChanged: &changed,
	}) {
		t.Fatalf("failed job should not report a tree change")
	}
	if ChangedTree(FirestoreJobStatus{
		Status: FirestoreJobStatusStateSucceeded,
	}) {
		t.Fatalf("succeeded job without treeChanged should not report a tree change")
	}
}
