// Package worker provides the API's client for dispatching jobs to the
// worker service over Connect RPC.
//
// This intentionally re-implements the dispatch client rather than importing
// apps/worker/pkg/worker, which pulls in the ADK / LLM / agent dependency
// tree. The API only needs to make two RPCs, so a small standalone client
// keeps the API binary lean and decoupled from worker internals. It mirrors
// apps/worker/pkg/worker.HTTPDispatcher; keep them in sync if the RPC
// surface changes.
package worker

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
	treev1connect "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1/treev1connect"
	"google.golang.org/api/idtoken"
)

// HTTPDispatcher dispatches plan generation / execution to the worker
// service over Connect. It satisfies service.WorkerDispatcher.
type HTTPDispatcher struct {
	baseURL string
}

func NewHTTPDispatcher(baseURL string) *HTTPDispatcher {
	return &HTTPDispatcher{baseURL: baseURL}
}

func (d *HTTPDispatcher) GenerateExecutionPlan(ctx context.Context, req domain.ExecutePlanRequest) error {
	httpClient, err := d.httpClient(ctx)
	if err != nil {
		return fmt.Errorf("http client: %w", err)
	}
	client := treev1connect.NewWorkerServiceClient(httpClient, strings.TrimRight(d.baseURL, "/"))
	rpcReq := connect.NewRequest(&treev1.GenerateExecutionPlanRequest{
		JobId:       req.JobID,
		JobType:     req.JobType,
		DocumentId:  req.DocumentID,
		WorkspaceId: req.WorkspaceID,
		TreeId:      req.TreeID,
		Filename:    req.Filename,
		MimeType:    req.MimeType,
	})
	if _, err = client.GenerateExecutionPlan(ctx, rpcReq); err != nil {
		return fmt.Errorf("GenerateExecutionPlan rpc: %w", err)
	}
	return nil
}

func (d *HTTPDispatcher) ExecuteApprovedPlan(ctx context.Context, req domain.ExecutePlanRequest) error {
	httpClient, err := d.httpClient(ctx)
	if err != nil {
		return fmt.Errorf("http client: %w", err)
	}
	client := treev1connect.NewWorkerServiceClient(httpClient, strings.TrimRight(d.baseURL, "/"))
	rpcReq := connect.NewRequest(&treev1.ExecuteApprovedPlanRequest{
		JobId:       req.JobID,
		JobType:     req.JobType,
		DocumentId:  req.DocumentID,
		WorkspaceId: req.WorkspaceID,
		TreeId:      req.TreeID,
		FileUri:     req.FileURI,
		Filename:    req.Filename,
		MimeType:    req.MimeType,
	})
	if _, err = client.ExecuteApprovedPlan(ctx, rpcReq); err != nil {
		if connect.CodeOf(err) == connect.CodeFailedPrecondition {
			return domain.ErrApprovalRequired
		}
		return err
	}
	return nil
}

// httpClient returns an ID-token authenticated client for Cloud Run
// (https targets), or the default client for local/plain-http targets.
func (d *HTTPDispatcher) httpClient(ctx context.Context) (*http.Client, error) {
	baseURL := strings.TrimRight(d.baseURL, "/")
	if !strings.HasPrefix(baseURL, "https://") {
		return http.DefaultClient, nil
	}
	return idtoken.NewClient(ctx, baseURL)
}
