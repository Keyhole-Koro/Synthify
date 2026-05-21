package handler

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/middleware"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	"github.com/synthify/backend/apps/api/internal/service"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

func TestUserHandler_SignInUser(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	// Mock billing as nil for simplicity here
	userSvc := service.NewUserService(store, store, nil, nil)
	h := NewUserHandler(userSvc)

	t.Run("requires authentication", func(t *testing.T) {
		resp, err := h.SignInUser(ctx, connect.NewRequest(&appv1.SignInUserRequest{}))
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("provisions authenticated user", func(t *testing.T) {
		userID := "owner"
		email := "owner@example.com"
		authedCtx := middleware.ContextWithUser(ctx, middleware.AuthUser{ID: userID, Email: email})

		resp, err := h.SignInUser(authedCtx, connect.NewRequest(&appv1.SignInUserRequest{}))

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, userID, resp.Msg.GetUser().GetUserId())
		assert.Equal(t, email, resp.Msg.GetUser().GetEmail())
		assert.True(t, resp.Msg.GetIsNewAccount())

		// Verify Account exists
		account, err := store.GetAccountByUser(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, userID, account.AccountID)
	})
}
