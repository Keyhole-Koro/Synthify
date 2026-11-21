package rpcvalidate_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	"github.com/synthify/backend/internal/gen/synthify/app/v1/appv1connect"
	"github.com/synthify/backend/internal/platform/transport/rpcvalidate"
)

func TestRequestRules(t *testing.T) {
	t.Parallel()
	validator, err := protovalidate.New()
	require.NoError(t, err)

	cases := []struct {
		name  string
		msg   proto.Message
		valid bool
	}{
		{"workspace name", &appv1.CreateWorkspaceRequest{Name: "Research"}, true},
		{"empty workspace name", &appv1.CreateWorkspaceRequest{Name: ""}, false},
		{"blank workspace name", &appv1.CreateWorkspaceRequest{Name: " \t "}, false},
		{"workspace id", &appv1.GetWorkspaceRequest{WorkspaceId: "ws-1"}, true},
		{"missing workspace id", &appv1.GetWorkspaceRequest{}, false},
		{"positive credit", &appv1.GrantCreditRequest{AccountId: "acc", AmountMinor: 100}, true},
		{"zero credit", &appv1.GrantCreditRequest{AccountId: "acc"}, false},
		{"missing target ref", &appv1.GetTreeEntityDetailRequest{}, false},
		{"target ref without id", &appv1.GetTreeEntityDetailRequest{TargetRef: &appv1.EntityRef{}}, false},
		{"target ref", &appv1.GetTreeEntityDetailRequest{TargetRef: &appv1.EntityRef{Id: "item-1"}}, true},
		{"usage event", &appv1.RecordUsageRequest{EventId: "ev", AccountId: "acc", Model: "m"}, true},
		{"usage event without model", &appv1.RecordUsageRequest{EventId: "ev", AccountId: "acc"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validator.Validate(tc.msg)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

type workspaceService struct {
	appv1connect.UnimplementedWorkspaceServiceHandler
	called bool
}

func (s *workspaceService) GetWorkspace(context.Context, *connect.Request[appv1.GetWorkspaceRequest]) (*connect.Response[appv1.GetWorkspaceResponse], error) {
	s.called = true
	return connect.NewResponse(&appv1.GetWorkspaceResponse{}), nil
}

func TestInterceptorRejectsInvalidRequestsBeforeTheHandler(t *testing.T) {
	t.Parallel()
	service := &workspaceService{}
	mux := http.NewServeMux()
	mux.Handle(appv1connect.NewWorkspaceServiceHandler(service, rpcvalidate.HandlerOptions()...))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := appv1connect.NewWorkspaceServiceClient(server.Client(), server.URL)

	_, err := client.GetWorkspace(context.Background(), connect.NewRequest(&appv1.GetWorkspaceRequest{}))
	var connectErr *connect.Error
	require.True(t, errors.As(err, &connectErr))
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	require.False(t, service.called)

	_, err = client.GetWorkspace(context.Background(), connect.NewRequest(&appv1.GetWorkspaceRequest{WorkspaceId: "ws-1"}))
	require.NoError(t, err)
	require.True(t, service.called)
}
