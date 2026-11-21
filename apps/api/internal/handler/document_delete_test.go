package handler

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/application"
	"github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type nopPrefixDeleter struct{}

func (nopPrefixDeleter) DeleteObjectsWithPrefix(context.Context, string) (int, error) {
	return 0, nil
}

func newDeletingDocumentHandler(store *mock.Store) *DocumentHandler {
	svc := application.NewDocumentService(application.DocumentServiceDeps{
		Repo:           store,
		Jobs:           store,
		LifecycleRepo:  store,
		Workspaces:     store,
		Tree:           store,
		Transactor:     store,
		Deletion:       store,
		ObjectPrefixes: nopPrefixDeleter{},
	})
	return NewDocumentHandler(svc, store)
}

func TestDocumentHandler_DeleteDocument_Unauthenticated(t *testing.T) {
	h := newDeletingDocumentHandler(mock.NewStore())

	resp, err := h.DeleteDocument(context.Background(), connect.NewRequest(&appv1.DeleteDocumentRequest{DocumentId: "doc"}))

	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestDocumentHandler_DeleteDocument(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner")
	h := newDeletingDocumentHandler(store)
	ownerCtx := auth.ContextWithPrincipal(ctx, auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "owner"})
	strangerCtx := auth.ContextWithPrincipal(ctx, auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "stranger"})
	doc, _, err := store.CreateDocument(ctx, fixture.Workspace.WorkspaceID, "owner", "a.pdf", "application/pdf", 10)
	require.NoError(t, err)

	t.Run("requires a document id", func(t *testing.T) {
		_, err := h.DeleteDocument(ownerCtx, connect.NewRequest(&appv1.DeleteDocumentRequest{}))
		assertConnectCode(t, err, connect.CodeInvalidArgument)
	})

	t.Run("refuses while the upload is open", func(t *testing.T) {
		_, err := h.DeleteDocument(ownerCtx, connect.NewRequest(&appv1.DeleteDocumentRequest{DocumentId: doc.DocumentID}))
		assertConnectCode(t, err, connect.CodeFailedPrecondition)
	})

	require.NoError(t, store.ConfirmDocumentUpload(ctx, doc.DocumentID, 10))

	t.Run("rejects another user", func(t *testing.T) {
		_, err := h.DeleteDocument(strangerCtx, connect.NewRequest(&appv1.DeleteDocumentRequest{DocumentId: doc.DocumentID}))
		assertConnectCode(t, err, connect.CodePermissionDenied)
	})

	t.Run("owner deletes it", func(t *testing.T) {
		resp, err := h.DeleteDocument(ownerCtx, connect.NewRequest(&appv1.DeleteDocumentRequest{DocumentId: doc.DocumentID}))
		require.NoError(t, err)
		assert.NotNil(t, resp)
		_, err = h.GetDocument(ownerCtx, connect.NewRequest(&appv1.GetDocumentRequest{DocumentId: doc.DocumentID}))
		assertConnectCode(t, err, connect.CodePermissionDenied)
	})
}
