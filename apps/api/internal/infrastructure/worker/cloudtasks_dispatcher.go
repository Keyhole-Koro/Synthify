package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/internal/platform/observability"
	"google.golang.org/protobuf/types/known/durationpb"
)

// defaultDispatchDeadline is used when the config leaves DispatchDeadline at
// zero. See config.WorkerDispatch.DispatchDeadline for the rationale (must be >=
// the worker's request timeout). 15m is the Cloud Tasks max for HTTP targets.
const defaultDispatchDeadline = 15 * time.Minute

// CloudTasksDispatcher enqueues worker invocations onto a Cloud Tasks queue
// instead of calling the worker synchronously. Cloud Tasks then drives the
// HTTP push to worker (waking its Cloud Run instance from min=0), and
// handles retries on transient failures.
//
// The dispatcher targets the worker's plain JSON /internal/dispatch-job
// endpoint (NOT the Connect RPCs) because Cloud Tasks delivers a raw HTTP
// body that does not match Connect's framing.
type CloudTasksDispatcher struct {
	client           *cloudtasks.Client
	queuePath        string // "projects/X/locations/Y/queues/Z"
	dispatchURL      string // "https://worker-host/internal/dispatch-job"
	invokerSA        string // service account email Cloud Tasks should impersonate to mint the OIDC token
	oidcAudience     string // audience the OIDC token must claim. Defaults to dispatchURL's scheme+host.
	dispatchDeadline time.Duration
	logger           *slog.Logger
}

type CloudTasksDispatcherConfig struct {
	QueuePath    string
	DispatchURL  string
	InvokerSA    string
	OIDCAudience string // optional; defaults to scheme+host of DispatchURL
	// DispatchDeadline is the per-task response deadline. Zero falls back to
	// defaultDispatchDeadline.
	DispatchDeadline time.Duration
	Logger           *slog.Logger
}

// NewCloudTasksDispatcher wires up the dispatcher and the underlying Cloud
// Tasks client. The client is long-lived — callers should reuse the
// dispatcher for the lifetime of the process and close it on shutdown via
// Close().
func NewCloudTasksDispatcher(ctx context.Context, cfg CloudTasksDispatcherConfig) (*CloudTasksDispatcher, error) {
	if cfg.QueuePath == "" {
		return nil, fmt.Errorf("queue path is required")
	}
	if cfg.DispatchURL == "" {
		return nil, fmt.Errorf("dispatch url is required")
	}
	if cfg.InvokerSA == "" {
		return nil, fmt.Errorf("invoker service account is required")
	}
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("cloud tasks client: %w", err)
	}
	audience := cfg.OIDCAudience
	if audience == "" {
		audience = audienceFromURL(cfg.DispatchURL)
	}
	deadline := cfg.DispatchDeadline
	if deadline <= 0 {
		deadline = defaultDispatchDeadline
	}
	cfg.Logger.Info("worker.cloudtasks_dispatcher_init",
		"queue", cfg.QueuePath,
		"dispatch_url", cfg.DispatchURL,
		"invoker_sa", cfg.InvokerSA,
		"oidc_audience", audience,
		"dispatch_deadline", deadline.String(),
	)
	return &CloudTasksDispatcher{
		client:           client,
		queuePath:        cfg.QueuePath,
		dispatchURL:      cfg.DispatchURL,
		invokerSA:        cfg.InvokerSA,
		oidcAudience:     audience,
		dispatchDeadline: deadline,
		logger:           cfg.Logger,
	}, nil
}

func (d *CloudTasksDispatcher) Close() error {
	return d.client.Close()
}

func (d *CloudTasksDispatcher) GenerateExecutionPlan(ctx context.Context, req domain.ExecutePlanRequest) error {
	return d.enqueue(ctx, "GenerateExecutionPlan", req)
}

func (d *CloudTasksDispatcher) ExecuteApprovedPlan(ctx context.Context, req domain.ExecutePlanRequest) error {
	return d.enqueue(ctx, "ExecuteApprovedPlan", req)
}

type dispatchBody struct {
	Procedure string                    `json:"procedure"`
	Payload   domain.ExecutePlanRequest `json:"payload"`
}

func (d *CloudTasksDispatcher) enqueue(ctx context.Context, procedure string, req domain.ExecutePlanRequest) error {
	body, err := json.Marshal(dispatchBody{Procedure: procedure, Payload: req})
	if err != nil {
		return fmt.Errorf("marshal dispatch body: %w", err)
	}

	// Naming the task after (job_id, procedure) gives Cloud Tasks free
	// deduplication: if the API retries a dispatch we have already enqueued
	// (e.g. because StartProcessing was re-driven on API restart), the
	// second CreateTask returns AlreadyExists and we treat it as success.
	taskName := fmt.Sprintf("%s/tasks/%s", d.queuePath, taskID(req.JobID, procedure))

	httpHeaders := make(http.Header)
	httpHeaders.Set("Content-Type", "application/json")
	observability.InjectTraceContext(ctx, httpHeaders)
	taskHeaders := make(map[string]string, len(httpHeaders))
	for key, values := range httpHeaders {
		if len(values) > 0 {
			taskHeaders[key] = values[0]
		}
	}

	task := &taskspb.Task{
		Name:             taskName,
		DispatchDeadline: durationpb.New(d.dispatchDeadline),
		MessageType: &taskspb.Task_HttpRequest{
			HttpRequest: &taskspb.HttpRequest{
				HttpMethod: taskspb.HttpMethod_POST,
				Url:        d.dispatchURL,
				Headers:    taskHeaders,
				Body:       body,
				AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
					OidcToken: &taskspb.OidcToken{
						ServiceAccountEmail: d.invokerSA,
						Audience:            d.oidcAudience,
					},
				},
			},
		},
	}

	if _, err := d.client.CreateTask(ctx, &taskspb.CreateTaskRequest{
		Parent: d.queuePath,
		Task:   task,
	}); err != nil {
		// AlreadyExists is the dedup happy path; surface anything else.
		if isAlreadyExists(err) {
			d.logger.Info("worker.cloudtasks_dispatcher.dedup",
				"procedure", procedure,
				"job_id", req.JobID,
				"task_name", taskName,
			)
			return nil
		}
		d.logger.Error("worker.cloudtasks_dispatcher.enqueue_failed",
			"procedure", procedure,
			"job_id", req.JobID,
			"error", err.Error(),
		)
		return fmt.Errorf("enqueue %s for job %s: %w", procedure, req.JobID, err)
	}
	d.logger.Info("worker.cloudtasks_dispatcher.enqueued",
		"procedure", procedure,
		"job_id", req.JobID,
		"task_name", taskName,
	)
	return nil
}

// taskID is the Cloud Tasks dedup key. Task names must match
// [A-Za-z0-9_-]{1,500}, so we hash the (job_id, procedure) pair into a hex
// digest rather than passing the raw ids.
func taskID(jobID, procedure string) string {
	sum := sha256.Sum256([]byte(jobID + "|" + procedure))
	return hex.EncodeToString(sum[:])
}

func audienceFromURL(u string) string {
	// Strip the path so the audience claim matches what the worker's
	// Cloud Run service expects (scheme+host only).
	if idx := strings.Index(u[len("https://"):], "/"); idx > 0 && strings.HasPrefix(u, "https://") {
		return u[:len("https://")+idx]
	}
	if idx := strings.Index(u[len("http://"):], "/"); idx > 0 && strings.HasPrefix(u, "http://") {
		return u[:len("http://")+idx]
	}
	return u
}

func isAlreadyExists(err error) bool {
	// The Cloud Tasks client wraps gRPC errors; check by message rather than
	// pulling in google.golang.org/grpc/status just for this. AlreadyExists
	// is the only code we treat as a no-op here.
	return strings.Contains(err.Error(), "AlreadyExists")
}
