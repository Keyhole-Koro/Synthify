package observability

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/newrelic/go-agent/v3/newrelic"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestConnectHandlerOptionsAddsTransactionToContext(t *testing.T) {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("test"),
		newrelic.ConfigLicense(strings.Repeat("a", 40)),
		newrelic.ConfigEnabled(false),
	)
	if err != nil {
		t.Fatalf("newrelic app: %v", err)
	}
	defer app.Shutdown(0)

	handler := connect.NewUnaryHandler(
		"/test.v1.TestService/Test",
		func(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[emptypb.Empty], error) {
			if newrelic.FromContext(ctx) == nil {
				t.Fatal("expected New Relic transaction in Connect handler context")
			}
			return connect.NewResponse(&emptypb.Empty{}), nil
		},
		ConnectHandlerOptions(app)...,
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	client := connect.NewClient[emptypb.Empty, emptypb.Empty](
		server.Client(),
		server.URL+"/test.v1.TestService/Test",
	)
	if _, err := client.CallUnary(context.Background(), connect.NewRequest(&emptypb.Empty{})); err != nil {
		t.Fatalf("call unary: %v", err)
	}
}

func TestConnectOptionsAreEmptyWhenNewRelicDisabled(t *testing.T) {
	if got := ConnectHandlerOptions(nil); len(got) != 0 {
		t.Fatalf("ConnectHandlerOptions(nil) length = %d, want 0", len(got))
	}
	if got := ConnectClientOptions(nil); len(got) != 0 {
		t.Fatalf("ConnectClientOptions(nil) length = %d, want 0", len(got))
	}
}
