package metering

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	connect "connectrpc.com/connect"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	"github.com/synthify/backend/internal/gen/synthify/app/v1/appv1connect"
)

// recordUsageMaxAttempts is the total number of RecordUsage attempts (1 initial
// + retries) for transient failures. RecordUsage is idempotent on the API side
// (events upsert by event_id), so retrying a delivery that actually succeeded
// is safe. Kept small: the worker call sits on the LLM hot path and the job
// log already captures a final drop.
const recordUsageMaxAttempts = 3

// recordUsageRetryBackoff is the base delay between RecordUsage attempts. The
// nth retry waits n*backoff so a brief API redeploy/cold-start window is
// covered without stalling the pipeline for long.
const recordUsageRetryBackoff = 200 * time.Millisecond

// ErrBillingNotConfigured is returned by NewConnectReporterStrict when the
// billing pipeline cannot be wired (missing API base URL or service token).
// In production/staging this must abort startup rather than silently dropping
// usage; see config.Worker.RequiresBilling.
var ErrBillingNotConfigured = errors.New("metering: billing reporter not configured (missing API base URL or service token)")

// NewConnectReporter returns an llm.UsageReporter that ships usage events to
// the billing API over the Connect protocol. The internal service token is
// attached via the X-Synthify-Service-Token header on every request; the
// receiving handler requires it (middleware.IsServiceCall) before processing.
//
// apiBaseURL is the same URL the worker uses for other API RPCs (e.g.
// http://127.0.0.1:8080 in dev). serviceToken must match the API's
// SYNTHIFY_INTERNAL_SERVICE_TOKEN env var.
//
// When serviceToken is empty the returned reporter is a no-op; usage will be
// dropped silently with a single warn log at construction. This keeps local
// dev (where the token may not be configured) from breaking the worker.
func NewConnectReporter(apiBaseURL, serviceToken string, opts ...connect.ClientOption) llm.UsageReporter {
	if apiBaseURL == "" || serviceToken == "" {
		return noopReporter{}
	}
	client := appv1connect.NewBillingServiceClient(http.DefaultClient, apiBaseURL, opts...)
	return &connectReporter{client: client, token: serviceToken}
}

// NewConnectReporterStrict is like NewConnectReporter but treats a missing API
// base URL or service token as a fatal misconfiguration: it returns
// ErrBillingNotConfigured instead of a silent no-op. Callers in
// production/staging (config.Worker.RequiresBilling) use this so a deploy that
// would never bill aborts at startup rather than running for free. The wrapped
// error includes which field is missing to speed up diagnosis.
func NewConnectReporterStrict(apiBaseURL, serviceToken string, opts ...connect.ClientOption) (llm.UsageReporter, error) {
	switch {
	case apiBaseURL == "" && serviceToken == "":
		return nil, fmt.Errorf("%w: API base URL and service token are empty", ErrBillingNotConfigured)
	case apiBaseURL == "":
		return nil, fmt.Errorf("%w: API base URL is empty", ErrBillingNotConfigured)
	case serviceToken == "":
		return nil, fmt.Errorf("%w: service token is empty", ErrBillingNotConfigured)
	}
	client := appv1connect.NewBillingServiceClient(http.DefaultClient, apiBaseURL, opts...)
	return &connectReporter{client: client, token: serviceToken}, nil
}

type connectReporter struct {
	client appv1connect.BillingServiceClient
	token  string
}

func (r *connectReporter) RecordUsage(ctx context.Context, ev domain.UsageEvent) error {
	var lastErr error
	for attempt := 1; attempt <= recordUsageMaxAttempts; attempt++ {
		// Build a fresh request per attempt: a connect.Request must not be
		// reused across calls.
		req := connect.NewRequest(&appv1.RecordUsageRequest{
			AccountId:    ev.AccountID,
			WorkspaceId:  ev.WorkspaceID,
			JobId:        ev.JobID,
			Model:        ev.Model,
			InputTokens:  ev.InputTokens,
			OutputTokens: ev.OutputTokens,
			EventId:      ev.EventID,
		})
		req.Header().Set("X-Synthify-Service-Token", r.token)
		if _, err := r.client.RecordUsage(ctx, req); err != nil {
			lastErr = err
			if attempt < recordUsageMaxAttempts && isRetryableUsageError(err) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(attempt) * recordUsageRetryBackoff):
				}
				continue
			}
			return err
		}
		return nil
	}
	return lastErr
}

// isRetryableUsageError reports whether a RecordUsage failure is worth
// retrying. Only transient transport/availability codes qualify; auth,
// invalid-argument, and other terminal codes are returned immediately so we
// don't waste the hot path retrying a request that can never succeed.
func isRetryableUsageError(err error) bool {
	switch connect.CodeOf(err) {
	case connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeResourceExhausted, connect.CodeAborted:
		return true
	default:
		// connect.CodeOf returns CodeUnknown for non-connect errors (e.g. raw
		// transport/dial failures before the server responds); treat those as
		// transient too.
		return connect.CodeOf(err) == connect.CodeUnknown
	}
}

type noopReporter struct{}

func (noopReporter) RecordUsage(context.Context, domain.UsageEvent) error { return nil }
