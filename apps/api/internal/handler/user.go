package handler

import (
	"context"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/application"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type UserUsecase interface {
	SignInUser(ctx context.Context, userID, email, displayName string) (*application.SignInUserResult, error)
}

type UserHandler struct {
	service UserUsecase
}

func NewUserHandler(svc UserUsecase) *UserHandler {
	return &UserHandler{service: svc}
}

func (h *UserHandler) SignInUser(ctx context.Context, req *connect.Request[appv1.SignInUserRequest]) (*connect.Response[appv1.SignInUserResponse], error) {
	principal, err := requireUserPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	result, err := h.service.SignInUser(ctx, principal.SubjectID, principal.Email, principal.DisplayName)
	if err != nil {
		return nil, toError(err)
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
