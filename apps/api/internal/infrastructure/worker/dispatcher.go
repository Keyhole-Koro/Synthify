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
	"golang.org/x/oauth2"
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

// RunChatTurn dispatches one dialogue turn. Unlike document processing this is
// always a direct call rather than a Cloud Tasks enqueue: the queueing delay is
// invisible on a multi-minute job but reads as dead air before the first token
// of an answer.
func (d *HTTPDispatcher) RunChatTurn(ctx context.Context, req domain.ChatTurnRequest) error {
	audience := strings.TrimRight(d.baseURL, "/")
	httpClient, err := d.httpClient(ctx)
	if err != nil {
		d.logger.Error("worker.dispatcher_http_client_failed",
			"audience", audience,
			"rpc", "RunChatTurn",
			"error", err.Error(),
		)
		return fmt.Errorf("http client: %w", err)
	}
	client := workerv1connect.NewWorkerServiceClient(httpClient, audience, d.opts...)

	history := make([]*workerv1.ChatMessage, 0, len(req.History))
	for _, m := range req.History {
		history = append(history, &workerv1.ChatMessage{
			Role: chatRoleToProto(m.Role),
			Text: m.Text,
		})
	}
	candidates := make([]*workerv1.ContextPaper, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		candidates = append(candidates, &workerv1.ContextPaper{
			PaperId:     c.PaperID,
			Title:       c.Title,
			Description: c.Description,
			Content:     c.Content,
		})
	}

	rpcReq := connect.NewRequest(&workerv1.RunChatTurnRequest{
		UserId:            req.UserID,
		WorkspaceId:       req.WorkspaceID,
		ChatId:            req.ChatID,
		TurnId:            req.TurnID,
		PaperId:           req.PaperID,
		History:           history,
		Candidates:        candidates,
		PinnedContextIds:  req.PinnedContextIDs,
		AutoSelectContext: req.AutoSelectContext,
	})
	if _, err = client.RunChatTurn(ctx, rpcReq); err != nil {
		d.logger.Error("worker.dispatcher_rpc_failed",
			"rpc", "RunChatTurn",
			"audience", audience,
			"request_url", audience+workerv1connect.WorkerServiceRunChatTurnProcedure,
			"connect_code", connect.CodeOf(err).String(),
			"error", err.Error(),
			"chat_id", req.ChatID,
			"turn_id", req.TurnID,
		)
		return err
	}
	return nil
}

func chatRoleToProto(r domain.ChatRole) workerv1.ChatRole {
	if r == domain.ChatRoleAssistant {
		return workerv1.ChatRole_CHAT_ROLE_ASSISTANT
	}
	return workerv1.ChatRole_CHAT_ROLE_USER
}

// httpClient returns an ID-token authenticated client for Cloud Run
// (https targets), or the default client for local/plain-http targets.
//
// For HTTPS targets we explicitly build the oauth2.Transport ourselves
// (instead of idtoken.NewClient) so we can inject authProbeTransport as
// Base. authProbeTransport observes the request after oauth2.Transport
// has attached the Bearer token, which is the only way to confirm whether
// the Authorization header actually made it onto the wire.
func (d *HTTPDispatcher) httpClient(ctx context.Context) (*http.Client, error) {
	audience := strings.TrimRight(d.baseURL, "/")
	if !strings.HasPrefix(audience, "https://") {
		return http.DefaultClient, nil
	}
	ts, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: &oauth2.Transport{
			Source: ts,
			Base: &authProbeTransport{
				inner:    http.DefaultTransport,
				logger:   d.logger,
				audience: audience,
			},
		},
	}, nil
}

// authProbeTransport logs whether the outbound request carried an
// Authorization header. It only emits a record when the response is an
// HTTP error (>=400) or the transport itself failed, to keep happy-path
// traffic silent.
type authProbeTransport struct {
	inner    http.RoundTripper
	logger   *slog.Logger
	audience string
}

func (t *authProbeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	authHeader := req.Header.Get("Authorization")
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		t.logger.Error("worker.dispatcher_transport_failed",
			"audience", t.audience,
			"url", req.URL.String(),
			"auth_present", authHeader != "",
			"auth_len", len(authHeader),
			"error", err.Error(),
		)
		return resp, err
	}
	if resp.StatusCode >= 400 {
		t.logger.Error("worker.dispatcher_http_error",
			"audience", t.audience,
			"url", req.URL.String(),
			"http_status", resp.StatusCode,
			"auth_present", authHeader != "",
			"auth_len", len(authHeader),
			"auth_scheme", schemeOf(authHeader),
			"response_content_type", resp.Header.Get("Content-Type"),
		)
	}
	return resp, nil
}

func schemeOf(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	if idx := strings.IndexByte(authHeader, ' '); idx > 0 {
		return authHeader[:idx]
	}
	return "unknown"
}
