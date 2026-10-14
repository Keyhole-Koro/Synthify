// Package jobstatus provides job status models, Firestore notifiers, JSON schema validation, and lifecycle semantics.
package jobstatus

// IsInFlight reports whether the given job state is actively in progress (either queued or running).
func IsInFlight(state FirestoreJobStatusState) bool {
	return state == FirestoreJobStatusStateQueued || state == FirestoreJobStatusStateRunning
}

// IsSucceeded reports whether the given job state represents a successful completion.
func IsSucceeded(state FirestoreJobStatusState) bool {
	return state == FirestoreJobStatusStateSucceeded
}

// IsFailed reports whether the given job state represents a terminal failure.
func IsFailed(state FirestoreJobStatusState) bool {
	return state == FirestoreJobStatusStateFailed
}

// IsTerminal reports whether the given job state is finished and will not transition further (succeeded or failed).
func IsTerminal(state FirestoreJobStatusState) bool {
	return IsSucceeded(state) || IsFailed(state)
}

// ChangedTree reports whether the completed job successfully caused modifications to the workspace knowledge tree.
func ChangedTree(doc FirestoreJobStatus) bool {
	return IsSucceeded(doc.Status) && doc.TreeChanged != nil && *doc.TreeChanged
}
