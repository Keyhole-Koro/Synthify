package handler

import (
	"context"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/service"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type UserHandler struct {
	service *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{service: svc}
}

func (h *UserHandler) SyncUser(ctx context.Context, req *connect.Request[appv1.SyncUserRequest]) (*connect.Response[appv1.SyncUserResponse], error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	result, err := h.service.SyncUser(ctx, user.ID, user.Email, "") // displayName is empty for now if not available in context
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&appv1.SyncUserResponse{
		User:      toProtoUser(result.User),
		IsNewUser: result.IsNewUser,
	}), nil
}

func (h *UserHandler) GetMe(ctx context.Context, req *connect.Request[appv1.GetMeRequest]) (*connect.Response[appv1.GetMeResponse], error) {
	// Not strictly requested in the plan but part of the UserService in proto
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
