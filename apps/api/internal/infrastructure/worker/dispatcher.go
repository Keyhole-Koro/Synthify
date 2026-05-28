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
	"log/slog"
	"net/http"
	"strings"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/domain"
	workerv1 "github.com/synthify/backend/internal/gen/synthify/worker/v1"
	workerv1connect "github.com/synthify/backend/internal/gen/synthify/worker/v1/workerv1connect"
	"google.golang.org/api/idtoken"
)

// HTTPDispatcher dispatches plan generation / execution to the worker
// service over Connect. It satisfies service.WorkerDispatcher.
type HTTPDispatcher struct {
	baseURL string
	opts    []connect.ClientOption
	logger  *slog.Logger
}

func NewHTTPDispatcher(baseURL string, logger *slog.Logger, opts ...connect.ClientOption) *HTTPDispatcher {
	audience := strings.TrimRight(baseURL, "/")
	logger.Info("worker.dispatcher_init",
		"base_url", baseURL,
		"audience", audience,
		"id_token_auth", strings.HasPrefix(audience, "https://"),
	)
	return &HTTPDispatcher{baseURL: baseURL, opts: opts, logger: logger}
}

func (d *HTTPDispatcher) GenerateExecutionPlan(ctx context.Context, req domain.ExecutePlanRequest) error {
	audience := strings.TrimRight(d.baseURL, "/")
	httpClient, err := d.httpClient(ctx)
	if err != nil {
		d.logger.Error("worker.dispatcher_http_client_failed",
			"audience", audience,
			"rpc", "GenerateExecutionPlan",
			"error", err.Error(),
		)
		return fmt.Errorf("http client: %w", err)
	}
	client := workerv1connect.NewWorkerServiceClient(httpClient, audience, d.opts...)
	rpcReq := connect.NewRequest(&workerv1.GenerateExecutionPlanRequest{
		JobId:       req.JobID,
		JobType:     req.JobType,
		DocumentId:  req.DocumentID,
		WorkspaceId: req.WorkspaceID,
		TreeId:      req.TreeID,
		Filename:    req.Filename,
		MimeType:    req.MimeType,
	})
	if _, err = client.GenerateExecutionPlan(ctx, rpcReq); err != nil {
		d.logger.Error("worker.dispatcher_rpc_failed",
			"rpc", "GenerateExecutionPlan",
			"audience", audience,
			"request_url", audience+workerv1connect.WorkerServiceGenerateExecutionPlanProcedure,
			"connect_code", connect.CodeOf(err).String(),
			"error", err.Error(),
			"job_id", req.JobID,
		)
		return fmt.Errorf("GenerateExecutionPlan rpc: %w", err)
	}
	return nil
}

func (d *HTTPDispatcher) ExecuteApprovedPlan(ctx context.Context, req domain.ExecutePlanRequest) error {
	audience := strings.TrimRight(d.baseURL, "/")
	httpClient, err := d.httpClient(ctx)
	if err != nil {
		d.logger.Error("worker.dispatcher_http_client_failed",
			"audience", audience,
			"rpc", "ExecuteApprovedPlan",
			"error", err.Error(),
		)
		return fmt.Errorf("http client: %w", err)
	}
	client := workerv1connect.NewWorkerServiceClient(httpClient, audience, d.opts...)
	rpcReq := connect.NewRequest(&workerv1.ExecuteApprovedPlanRequest{
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
		code := connect.CodeOf(err)
		if code == connect.CodeFailedPrecondition {
			return domain.ErrApprovalRequired
		}
		d.logger.Error("worker.dispatcher_rpc_failed",
			"rpc", "ExecuteApprovedPlan",
			"audience", audience,
			"request_url", audience+workerv1connect.WorkerServiceExecuteApprovedPlanProcedure,
			"connect_code", code.String(),
			"error", err.Error(),
			"job_id", req.JobID,
		)
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
