package jobstatus

import (
	"context"
	"time"

	"log/slog"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
)

type Payload struct {
	JobID       string
	JobType     string
	DocumentID  string
	WorkspaceID string
	TreeID      string
}

type UpdateFields struct {
	CurrentStage                 *string
	Progress                     *int
	Message                      *string
	ErrorMessage                 *string
	SuggestedWorkspaceName       *string
	SuggestedWorkspaceNameSource *string
	StartedAt                    *string
	CompletedAt                  *string
}

func String(value string) *string {
	return &value
}

func Int(value int) *int {
	return &value
}

func (f UpdateFields) mapWithUpdatedAt() map[string]any {
	fields := map[string]any{FirestoreJobStatusFieldUpdatedAt: nowRFC3339()}
	if f.CurrentStage != nil {
		fields[FirestoreJobStatusFieldCurrentStage] = *f.CurrentStage
	}
	if f.Progress != nil {
		fields[FirestoreJobStatusFieldProgress] = *f.Progress
	}
	if f.Message != nil {
		fields[FirestoreJobStatusFieldMessage] = *f.Message
	}
	if f.ErrorMessage != nil {
		fields[FirestoreJobStatusFieldErrorMessage] = *f.ErrorMessage
	}
	if f.SuggestedWorkspaceName != nil {
		fields[FirestoreJobStatusFieldSuggestedWorkspaceName] = *f.SuggestedWorkspaceName
	}
	if f.SuggestedWorkspaceNameSource != nil {
		fields[FirestoreJobStatusFieldSuggestedWorkspaceNameSource] = *f.SuggestedWorkspaceNameSource
	}
	if f.StartedAt != nil {
		fields[FirestoreJobStatusFieldStartedAt] = *f.StartedAt
	}
	if f.CompletedAt != nil {
		fields[FirestoreJobStatusFieldCompletedAt] = *f.CompletedAt
	}
	return fields
}

type Notifier interface {
	Queued(ctx context.Context, payload Payload) error
	Running(ctx context.Context, payload Payload) error
	Stage(ctx context.Context, payload Payload, stage string) error
	StageProgress(ctx context.Context, payload Payload, stage string, progress int, message string) error
	UpdateFields(ctx context.Context, workspaceID, jobID string, fields UpdateFields) error
	Failed(ctx context.Context, payload Payload, errorMessage string) error
	Completed(ctx context.Context, payload Payload) error
}

type noopNotifier struct{}

func (noopNotifier) Queued(context.Context, Payload) error        { return nil }
func (noopNotifier) Running(context.Context, Payload) error       { return nil }
func (noopNotifier) Stage(context.Context, Payload, string) error { return nil }
func (noopNotifier) StageProgress(context.Context, Payload, string, int, string) error {
	return nil
}
func (noopNotifier) UpdateFields(context.Context, string, string, UpdateFields) error { return nil }
func (noopNotifier) Failed(context.Context, Payload, string) error                    { return nil }
func (noopNotifier) Completed(context.Context, Payload) error                         { return nil }

type firestoreNotifier struct {
	client *firestore.Client
	logger *slog.Logger
}

func NewNotifier(ctx context.Context, projectID string, logger *slog.Logger) Notifier {
	if projectID == "" {
		return noopNotifier{}
	}
	initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	app, err := firebase.NewApp(initCtx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		logger.Error("jobstatus.firebase_init_failed", "error", err.Error(), "project_id", projectID)
		return noopNotifier{}
	}
	client, err := app.Firestore(initCtx)
	if err != nil {
		logger.Error("jobstatus.firestore_init_failed", "error", err.Error(), "project_id", projectID)
		return noopNotifier{}
	}
	return &firestoreNotifier{client: client, logger: logger}
}

func (n *firestoreNotifier) Queued(ctx context.Context, payload Payload) error {
	return n.write(ctx, payload, map[string]any{
		FirestoreJobStatusFieldStatus:       string(FirestoreJobStatusStateQueued),
		FirestoreJobStatusFieldCurrentStage: "",
		FirestoreJobStatusFieldProgress:     0,
		FirestoreJobStatusFieldMessage:      "Queued",
		FirestoreJobStatusFieldErrorMessage: "",
		FirestoreJobStatusFieldCreatedAt:    nowRFC3339(),
		FirestoreJobStatusFieldUpdatedAt:    nowRFC3339(),
	})
}

func (n *firestoreNotifier) Running(ctx context.Context, payload Payload) error {
	return n.write(ctx, payload, map[string]any{
		FirestoreJobStatusFieldStatus:       string(FirestoreJobStatusStateRunning),
		FirestoreJobStatusFieldCurrentStage: "",
		FirestoreJobStatusFieldProgress:     5,
		FirestoreJobStatusFieldMessage:      "Processing started",
		FirestoreJobStatusFieldErrorMessage: "",
		FirestoreJobStatusFieldStartedAt:    nowRFC3339(),
		FirestoreJobStatusFieldUpdatedAt:    nowRFC3339(),
	})
}

func (n *firestoreNotifier) Stage(ctx context.Context, payload Payload, stage string) error {
	return n.StageProgress(ctx, payload, stage, -1, "")
}

func (n *firestoreNotifier) StageProgress(ctx context.Context, payload Payload, stage string, progress int, message string) error {
	fields := map[string]any{
		FirestoreJobStatusFieldStatus:       string(FirestoreJobStatusStateRunning),
		FirestoreJobStatusFieldCurrentStage: stage,
		FirestoreJobStatusFieldUpdatedAt:    nowRFC3339(),
	}
	if progress >= 0 {
		fields[FirestoreJobStatusFieldProgress] = progress
	}
	if message != "" {
		fields[FirestoreJobStatusFieldMessage] = message
	}
	return n.write(ctx, payload, fields)
}

func (n *firestoreNotifier) Failed(ctx context.Context, payload Payload, errorMessage string) error {
	return n.write(ctx, payload, map[string]any{
		FirestoreJobStatusFieldStatus:       string(FirestoreJobStatusStateFailed),
		FirestoreJobStatusFieldCurrentStage: "",
		FirestoreJobStatusFieldMessage:      "Failed",
		FirestoreJobStatusFieldErrorMessage: errorMessage,
		FirestoreJobStatusFieldUpdatedAt:    nowRFC3339(),
		FirestoreJobStatusFieldCompletedAt:  nowRFC3339(),
		// Bound the lifetime so the workspace's jobs collection does not
		// grow without bound; Firestore TTL policy deletes the document
		// once expiresAt passes.
		FirestoreJobStatusFieldExpiresAt: ttlFromNow(),
	})
}

func (n *firestoreNotifier) UpdateFields(ctx context.Context, workspaceID, jobID string, fields UpdateFields) error {
	if workspaceID == "" || jobID == "" {
		return nil
	}
	doc := fields.mapWithUpdatedAt()
	if len(doc) == 1 {
		return nil
	}
	_, err := n.client.Collection("workspaces").Doc(workspaceID).Collection("jobs").Doc(jobID).Set(ctx, doc, firestore.MergeAll)
	if err != nil {
		n.logger.Error("jobstatus.firestore_patch_failed", "error", err.Error(), "job_id", jobID)
		return err
	}
	return nil
}

func (n *firestoreNotifier) Completed(ctx context.Context, payload Payload) error {
	return n.write(ctx, payload, map[string]any{
		FirestoreJobStatusFieldStatus:       string(FirestoreJobStatusStateSucceeded),
		FirestoreJobStatusFieldCurrentStage: "",
		FirestoreJobStatusFieldProgress:     100,
		FirestoreJobStatusFieldMessage:      "Completed",
		FirestoreJobStatusFieldErrorMessage: "",
		FirestoreJobStatusFieldUpdatedAt:    nowRFC3339(),
		FirestoreJobStatusFieldCompletedAt:  nowRFC3339(),
		// Same TTL contract as Failed: keep the document for a week so the
		// user can still inspect a finished job, then let TTL clean it up.
		FirestoreJobStatusFieldExpiresAt: ttlFromNow(),
	})
}

func (n *firestoreNotifier) write(ctx context.Context, payload Payload, fields map[string]any) error {
	doc := map[string]any{
		FirestoreJobStatusFieldJobID:       payload.JobID,
		FirestoreJobStatusFieldJobType:     payload.JobType,
		FirestoreJobStatusFieldDocumentID:  payload.DocumentID,
		FirestoreJobStatusFieldWorkspaceID: payload.WorkspaceID,
		FirestoreJobStatusFieldTreeID:      payload.TreeID,
	}
	for key, value := range fields {
		doc[key] = value
	}
	if err := validateFirestoreJobStatus(doc); err != nil {
		n.logger.Error("jobstatus.firestore_schema_validation_failed", "error", err.Error(), "job_id", payload.JobID)
		return err
	}
	_, err := n.client.Collection("workspaces").Doc(payload.WorkspaceID).Collection("jobs").Doc(payload.JobID).Set(ctx, doc, firestore.MergeAll)
	if err != nil {
		n.logger.Error("jobstatus.firestore_write_failed", "error", err.Error(), "job_id", payload.JobID)
		return err
	}
	return nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// jobStatusTTL is how long a terminal job status document survives in
// Firestore before the platform TTL policy garbage-collects it. The
// expiresAt field is bound to the TTL policy in terraform; changing the
// duration here only takes effect for documents written after the change.
const jobStatusTTL = 7 * 24 * time.Hour

func ttlFromNow() time.Time {
	return time.Now().UTC().Add(jobStatusTTL)
}
