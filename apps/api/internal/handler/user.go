package handler

import (
	"context"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/service"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type UserHandler struct {
	service service.UserUsecase
}

func NewUserHandler(svc service.UserUsecase) *UserHandler {
	return &UserHandler{service: svc}
}

func (h *UserHandler) SignInUser(ctx context.Context, req *connect.Request[appv1.SignInUserRequest]) (*connect.Response[appv1.SignInUserResponse], error) {
	user, err := requireAuthUser(ctx)
	if err != nil {
		return nil, err
	}

	// displayName は Firebase 側のクレームに含まれていないため当面空のまま。
	// 将来 ID トークンの name クレームから引くなら middleware で AuthUser に詰めること。
	result, err := h.service.SignInUser(ctx, user.ID, user.Email, "")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&appv1.SignInUserResponse{
		User:         toProtoUser(result.User),
		IsNewAccount: result.IsNewAccount,
	}), nil
}

func (h *UserHandler) GetMe(ctx context.Context, req *connect.Request[appv1.GetMeRequest]) (*connect.Response[appv1.GetMeResponse], error) {
	// Not strictly requested in the plan but part of the UserService in proto
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
