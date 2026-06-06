package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
)

type fakeImageURLIssuer struct {
	gotWorkspace string
	gotDocument  string
	gotFilename  string
}

func (f *fakeImageURLIssuer) IssueDocumentImageURL(ctx context.Context, workspaceID, documentID, filename string) (repository.SignedURL, error) {
	f.gotWorkspace = workspaceID
	f.gotDocument = documentID
	f.gotFilename = filename
	return repository.SignedURL{URL: "https://signed.example/" + documentID}, nil
}

func newImageURLService(t *testing.T, store *mock.Store, issuer repository.DocumentImageURLIssuer) *DocumentService {
	t.Helper()
	return NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ImageURLIssuer:   issuer,
		Logger:           discardLogger(),
	})
}

// setupImageFile creates an owner, workspace, document, and a document file,
// returning the file id and workspace id for image-URL tests.
func setupImageFile(t *testing.T, store *mock.Store, owner string) (fileID, workspaceID string) {
	t.Helper()
	ctx := context.Background()
	account, err := store.GetOrCreateAccount(ctx, owner)
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	doc, _, err := store.CreateDocument(ctx, ws.WorkspaceID, owner, "diagram.png", "image/png", 1024)
	require.NoError(t, err)
	file, err := store.CreateDocumentFile(ctx, doc.DocumentID, "diagram.png", "image/png", 1024)
	require.NoError(t, err)
	return file.FileID, ws.WorkspaceID
}

func TestIssueImageURL_OwnerGetsSignedURL(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fileID, _ := setupImageFile(t, store, "owner")
	issuer := &fakeImageURLIssuer{}
	svc := newImageURLService(t, store, issuer)

	signed, err := svc.IssueImageURL(ctx, fileID, "owner")

	require.NoError(t, err)
	assert.NotEmpty(t, signed.URL)
	// The issuer must receive the file's own coordinates (filename included so it
	// can build the source object key).
	assert.Equal(t, "diagram.png", issuer.gotFilename)
	assert.NotEmpty(t, issuer.gotWorkspace)
	assert.NotEmpty(t, issuer.gotDocument)
}

func TestIssueImageURL_NonMemberForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fileID, _ := setupImageFile(t, store, "owner")
	// "intruder" has no access to the owner's workspace.
	_, err := store.GetOrCreateAccount(ctx, "intruder")
	require.NoError(t, err)
	svc := newImageURLService(t, store, &fakeImageURLIssuer{})

	_, err = svc.IssueImageURL(ctx, fileID, "intruder")

	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestIssueImageURL_UnknownFileForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	svc := newImageURLService(t, store, &fakeImageURLIssuer{})

	// A missing file must not leak existence: it returns Forbidden, not NotFound.
	_, err := svc.IssueImageURL(ctx, "does-not-exist", "owner")

	assert.ErrorIs(t, err, domain.ErrForbidden)
}
