package domain

import "errors"

// ErrDocumentBusy is returned when a document cannot be deleted yet because its
// upload or a processing job is still in flight. Either would keep writing rows
// and stored files for the document after it is gone, so the caller waits and
// retries instead.
var ErrDocumentBusy = errors.New("document is still being uploaded or processed")

// DocumentDeletionState is what deleting a document has to know beforehand.
type DocumentDeletionState struct {
	// JobIDs lists every processing job of the document. Each may have left
	// stage checkpoints in storage, which are keyed by job, not by document.
	JobIDs []string
	// UploadPending is true while the document's upload reservation is open,
	// i.e. a signed upload URL issued for it may still be in use.
	UploadPending bool
}

// WorkspaceStorageOwners lists the documents and processing jobs of a deleted
// workspace. The worker stores its cache per document and its checkpoints per
// job, outside the workspace's own "{ws}/" prefix, so purging the workspace
// has to know them.
type WorkspaceStorageOwners struct {
	DocumentIDs []string
	JobIDs      []string
}

// ReparentAroundDeleted applies the tree rule for deleting nodes: a kept node
// whose parent is deleted moves up to its nearest kept ancestor instead of
// jumping to the workspace root. parentOf maps every node of one workspace to
// its parent ("" = the workspace root). The result maps each node to move to
// its new parent ("" = root). A node is never moved under a node outside
// parentOf, and a parent cycle among deleted nodes ends at the root.
func ReparentAroundDeleted(parentOf map[string]string, deleted map[string]bool) map[string]string {
	moves := map[string]string{}
	for id, parent := range parentOf {
		if deleted[id] || !deleted[parent] {
			continue
		}
		ancestor := parent
		for steps := 0; deleted[ancestor] && steps <= len(parentOf); steps++ {
			ancestor = parentOf[ancestor]
		}
		if _, inTree := parentOf[ancestor]; !inTree || deleted[ancestor] {
			ancestor = ""
		}
		moves[id] = ancestor
	}
	return moves
}
