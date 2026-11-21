package repository

import (
	"context"

	"github.com/synthify/backend/apps/api/internal/domain"
)

// DocumentDeletionRepository removes a document's rows. Its stored files go
// first, through ObjectPrefixDeleter: if that fails the rows stay, the document
// stays listed, and the user can retry. The reverse order would leave files no
// row points to any more.
type DocumentDeletionRepository interface {
	// GetDocumentDeletionState returns the document's processing job IDs and
	// whether its upload is still open. ErrNotFound if the document is gone.
	GetDocumentDeletionState(ctx context.Context, documentID string) (*domain.DocumentDeletionState, error)
	// DeleteDocumentRows deletes, in one transaction:
	//   - the tree nodes that come from this document and no other (they cite
	//     it as a source, or its processing jobs created or edited them);
	//     their kept children move up to the nearest kept ancestor,
	//   - the document row and everything that cascades from it (files,
	//     chunks, upload reservation, processing jobs with their logs and
	//     checkpoints, sources),
	//   - the document's confirmed bytes from the account's storage usage.
	// Nodes another document also contributed to stay and lose only this
	// document's sources. Nodes made by hand stay, and so do nodes a person
	// has curated or locked (governance_state human_curated or locked): they
	// are protected from automated changes, and stay as hand-owned nodes
	// without this document's sources. It returns the number of nodes
	// deleted, and ErrNotFound if the document is already gone.
	DeleteDocumentRows(ctx context.Context, documentID string) (deletedItems int, err error)
}

// ObjectPrefixDeleter deletes every stored object whose name starts with a
// prefix, including the noncurrent versions bucket versioning keeps, so the
// data is gone rather than hidden. Prefixes come from
// internal/platform/storage (WorkspaceObjectPrefix and friends); anything else
// is refused.
type ObjectPrefixDeleter interface {
	DeleteObjectsWithPrefix(ctx context.Context, prefix string) (deleted int, err error)
}
