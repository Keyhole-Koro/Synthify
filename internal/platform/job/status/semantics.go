package jobstatus

func IsInFlight(state FirestoreJobStatusState) bool {
	return state == FirestoreJobStatusStateQueued || state == FirestoreJobStatusStateRunning
}

func IsSucceeded(state FirestoreJobStatusState) bool {
	return state == FirestoreJobStatusStateSucceeded
}

func IsFailed(state FirestoreJobStatusState) bool {
	return state == FirestoreJobStatusStateFailed
}

func IsTerminal(state FirestoreJobStatusState) bool {
	return IsSucceeded(state) || IsFailed(state)
}

func HasCreatedDocumentRoot(doc FirestoreJobStatus) bool {
	return IsSucceeded(doc.Status) && doc.CreatedDocumentRootItemID != nil && *doc.CreatedDocumentRootItemID != ""
}
