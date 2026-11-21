package repository

import (
	"context"
	"time"

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
	// document's sources. Nodes made by hand stay. It returns the number of
	// nodes deleted, and ErrNotFound if the document is already gone.
	DeleteDocumentRows(ctx context.Context, documentID string) (deletedItems int, err error)
}

// WorkspacePurgeRepository finds workspaces that were deleted (DeleteWorkspace
// only sets deleted_at) and removes their rows for good. As for documents, the
// stored files go first, through ObjectPrefixDeleter.
type WorkspacePurgeRepository interface {
	// ListWorkspacesDeletedBefore returns up to limit workspaces deleted at or
	// before cutoff, oldest deletion first.
	ListWorkspacesDeletedBefore(ctx context.Context, cutoff time.Time, limit int) ([]string, error)
	// GetWorkspaceStorageOwners returns the document and job IDs of a deleted
	// workspace. ErrNotFound if no deleted workspace has that ID.
	GetWorkspaceStorageOwners(ctx context.Context, workspaceID string) (*domain.WorkspaceStorageOwners, error)
	// PurgeWorkspaceRows deletes a workspace that is marked deleted, with
	// every row that cascades from it (documents and everything under them,
	// the tree, members, share links, jobs, dynamic tools), and gives its
	// confirmed upload bytes back to the account. It returns false, deleting
	// nothing, when the workspace is not marked deleted or is already gone.
	PurgeWorkspaceRows(ctx context.Context, workspaceID string) (bool, error)
}

// AccountDeletionRepository supports deleting a user's account. The user's own
// workspaces are marked deleted and purged first (WorkspacePurgeRepository),
// so the account rows go last.
type AccountDeletionRepository interface {
	// GetAccountDeletionPlan returns the accounts the user alone belongs to,
	// their workspaces, the other workspaces the user can reach, and whether a
	// paid plan or a document upload or job still in flight stands in the way. A job counts as in flight only if it
	// moved after jobsActiveSince.
	GetAccountDeletionPlan(ctx context.Context, userID string, jobsActiveSince, now time.Time) (*domain.AccountDeletionPlan, error)
	// MarkWorkspacesDeleted sets deleted_at on the workspaces not deleted yet.
	MarkWorkspacesDeleted(ctx context.Context, workspaceIDs []string, now time.Time) error
	// DeleteAccountRows deletes, in one transaction, the user's memberships in
	// other workspaces, the user's ID from what they added to those workspaces
	// (documents, jobs, nodes), the accounts they alone belong to with
	// everything that cascades from them (usage, credits, invoices, payment
	// methods), their membership of shared accounts, and the users row. It
	// fails with ErrConflict while one of their accounts still has a
	// workspace, so a workspace created meanwhile is not orphaned.
	DeleteAccountRows(ctx context.Context, userID string) error
}

// ObjectPrefixDeleter deletes every stored object whose name starts with a
// prefix, including the noncurrent versions bucket versioning keeps, so the
// data is gone rather than hidden. Prefixes come from
// internal/platform/storage (WorkspaceObjectPrefix and friends); anything else
// is refused.
type ObjectPrefixDeleter interface {
	DeleteObjectsWithPrefix(ctx context.Context, prefix string) (deleted int, err error)
}
