package worker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
)

// billingAccountFor は usage の課金先 account を決める。本人 (requestedBy) の
// account 負担が原則で、引けないときは workspace 所有者にフォールバックする。
func newBillingTestWorker(t *testing.T) (*Worker, *mock.Store, string) {
	t.Helper()
	ctx := context.Background()
	store := mock.NewStore()
	// owner が workspace を作る (mock では account_id == user_id)。
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner")
	w := &Worker{repo: store, logger: newDiscardLogger()}
	return w, store, fixture.Workspace.WorkspaceID
}

func TestBillingAccountFor_Requester_ChargesRequesterAccount(t *testing.T) {
	ctx := context.Background()
	w, store, wsID := newBillingTestWorker(t)
	// 招待メンバー本人の account を用意する (owner とは別 account)。
	_, err := store.GetOrCreateAccount(ctx, "invitee")
	require.NoError(t, err)

	got := w.billingAccountFor(ctx, "invitee", wsID)
	assert.Equal(t, "invitee", got, "cost is borne by the requesting user's account")
}

func TestBillingAccountFor_System_FallsBackToOwner(t *testing.T) {
	ctx := context.Background()
	w, _, wsID := newBillingTestWorker(t)

	// 自動再開など requestedBy が "system" の場合は所有者 account に倒す。
	got := w.billingAccountFor(ctx, "system", wsID)
	assert.Equal(t, "owner", got, "system jobs fall back to the workspace owner")
}

func TestBillingAccountFor_Empty_FallsBackToOwner(t *testing.T) {
	ctx := context.Background()
	w, _, wsID := newBillingTestWorker(t)

	got := w.billingAccountFor(ctx, "", wsID)
	assert.Equal(t, "owner", got, "empty requestedBy falls back to the workspace owner")
}

func TestBillingAccountFor_UnresolvableRequester_FallsBackToOwner(t *testing.T) {
	ctx := context.Background()
	w, _, wsID := newBillingTestWorker(t)

	// requestedBy のユーザーに account が無い (退会済み等) ときも所有者に倒す。
	got := w.billingAccountFor(ctx, "ghost", wsID)
	assert.Equal(t, "owner", got, "unresolvable requester falls back to the workspace owner")
}

func TestBillingAccountFor_NoWorkspace_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	w, _, _ := newBillingTestWorker(t)

	// 本人 account も所有者 workspace も引けなければ空文字 (usage は drop)。
	got := w.billingAccountFor(ctx, "ghost", "ws-nonexistent")
	assert.Empty(t, got, "unresolvable account drops usage rather than failing")
}
