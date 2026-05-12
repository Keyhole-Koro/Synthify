package jobstatus

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/synthify/backend/packages/shared/applog"
)

type Payload struct {
	JobID       string
	JobType     string
	DocumentID  string
	WorkspaceID string
	TreeID      string
}

type Notifier interface {
	Queued(ctx context.Context, payload Payload) error
	Running(ctx context.Context, payload Payload) error
	Stage(ctx context.Context, payload Payload, stage string) error
	StageProgress(ctx context.Context, payload Payload, stage string, progress int, message string) error
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
func (noopNotifier) Failed(context.Context, Payload, string) error { return nil }
func (noopNotifier) Completed(context.Context, Payload) error      { return nil }

type firestoreNotifier struct {
	client *firestore.Client
	logger applog.Logger
}

func NewNotifier(ctx context.Context, projectID string, logger applog.Logger) Notifier {
	if projectID == "" {
		return noopNotifier{}
	}
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	app, err := firebase.NewApp(initCtx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		logger.Error(ctx, "jobstatus.firebase_init_failed", err, map[string]any{"project_id": projectID})
		return noopNotifier{}
	}
	client, err := app.Firestore(initCtx)
	if err != nil {
		logger.Error(ctx, "jobstatus.firestore_init_failed", err, map[string]any{"project_id": projectID})
		return noopNotifier{}
	}
	return &firestoreNotifier{client: client, logger: logger}
}

func (n *firestoreNotifier) Queued(ctx context.Context, payload Payload) error {
	return n.write(ctx, payload, map[string]any{
		"status":       "queued",
		"currentStage": "",
		"progress":     0,
		"message":      "Queued",
		"errorMessage": "",
		"createdAt":    nowRFC3339(),
		"updatedAt":    nowRFC3339(),
	})
}

func (n *firestoreNotifier) Running(ctx context.Context, payload Payload) error {
	return n.write(ctx, payload, map[string]any{
		"status":       "running",
		"progress":     5,
		"message":      "Processing started",
		"errorMessage": "",
		"startedAt":    nowRFC3339(),
		"updatedAt":    nowRFC3339(),
	})
}

func (n *firestoreNotifier) Stage(ctx context.Context, payload Payload, stage string) error {
	return n.StageProgress(ctx, payload, stage, -1, "")
}

func (n *firestoreNotifier) StageProgress(ctx context.Context, payload Payload, stage string, progress int, message string) error {
	fields := map[string]any{
		"status":       "running",
		"currentStage": stage,
		"updatedAt":    nowRFC3339(),
	}
	if progress >= 0 {
		fields["progress"] = progress
	}
	if message != "" {
		fields["message"] = message
	}
	return n.write(ctx, payload, fields)
}

func (n *firestoreNotifier) Failed(ctx context.Context, payload Payload, errorMessage string) error {
	return n.write(ctx, payload, map[string]any{
		"status":       "failed",
		"currentStage": "",
		"message":      "Failed",
		"errorMessage": errorMessage,
		"updatedAt":    nowRFC3339(),
		"completedAt":  nowRFC3339(),
	})
}

func (n *firestoreNotifier) Completed(ctx context.Context, payload Payload) error {
	return n.write(ctx, payload, map[string]any{
		"status":       "succeeded",
		"currentStage": "",
		"progress":     100,
		"message":      "Completed",
		"errorMessage": "",
		"updatedAt":    nowRFC3339(),
		"completedAt":  nowRFC3339(),
	})
}

func (n *firestoreNotifier) write(ctx context.Context, payload Payload, fields map[string]any) error {
	doc := map[string]any{
		"jobId":       payload.JobID,
		"jobType":     payload.JobType,
		"documentId":  payload.DocumentID,
		"workspaceId": payload.WorkspaceID,
		"treeId":      payload.TreeID,
	}
	for key, value := range fields {
		doc[key] = value
	}
	_, err := n.client.Collection("workspaces").Doc(payload.WorkspaceID).Collection("jobs").Doc(payload.JobID).Set(ctx, doc, firestore.MergeAll)
	if err != nil {
		n.logger.Error(ctx, "jobstatus.firestore_write_failed", err, map[string]any{"job_id": payload.JobID})
		return err
	}
	return nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
