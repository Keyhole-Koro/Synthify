package bootstrap

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/application"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
)

// deletedObjects records the document objects the sweep deletes.
type deletedObjects []string

func (d *deletedObjects) DeleteDocumentObject(_ context.Context, workspaceID, documentID string) error {
	*d = append(*d, workspaceID+"/"+documentID)
	return nil
}

// The periodic sweep expires reservations that ran out without being
// confirmed and deletes what was uploaded for them, and leaves the others.
func TestExpireUploadReservations_DeletesAbandonedUploadsOnly(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner")
	var deleted deletedObjects
	svc := application.NewDocumentService(application.DocumentServiceDeps{
		Repo:          store,
		Jobs:          store,
		LifecycleRepo: store,
		Workspaces:    store,
		Tree:          store,
		Transactor:    store,
		ObjectStore:   &deleted,
		Logger:        slog.New(slog.DiscardHandler),
	})
	wsID := fixture.Workspace.WorkspaceID
	abandoned, _, err := svc.CreateDocument(ctx, wsID, "owner", "abandoned.pdf", "application/pdf", 10)
	require.NoError(t, err)
	confirmed, _, err := svc.CreateDocument(ctx, wsID, "owner", "confirmed.pdf", "application/pdf", 10)
	require.NoError(t, err)
	require.NoError(t, store.ConfirmDocumentUpload(ctx, confirmed.DocumentID, 10))

	expireUploadReservations(svc, time.Now().Add(time.Minute))
	assert.Empty(t, deleted, "nothing has run out yet")

	expireUploadReservations(svc, time.Now().Add(16*time.Minute))
	assert.Equal(t, deletedObjects{wsID + "/" + abandoned.DocumentID}, deleted)

	expireUploadReservations(svc, time.Now().Add(30*time.Minute))
	assert.Len(t, deleted, 1, "an expired reservation is swept once")
}
