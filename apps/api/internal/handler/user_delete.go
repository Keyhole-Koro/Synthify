package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// AccountDeletionUsecase deletes the caller's account.
type AccountDeletionUsecase interface {
	DeleteAccount(ctx context.Context, userID string) error
}

// WithAccountDeletion sets what DeleteAccount uses. Without it the RPC
// answers Unimplemented.
func (h *UserHandler) WithAccountDeletion(svc AccountDeletionUsecase) *UserHandler {
	h.accountDeletion = svc
	return h
}

// DeleteAccount deletes the caller's own account. The user ID comes from the
// verified ID token only, so nobody can name another user's account.
func (h *UserHandler) DeleteAccount(ctx context.Context, req *connect.Request[appv1.DeleteAccountRequest]) (*connect.Response[appv1.DeleteAccountResponse], error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if h.accountDeletion == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("account deletion is not configured"))
	}
	if err := h.accountDeletion.DeleteAccount(ctx, userID); err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.DeleteAccountResponse{}), nil
}
