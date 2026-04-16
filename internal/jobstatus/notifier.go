package jobstatus

import (
	"context"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
)

type Payload struct {
	JobID       string
	JobType     string
	DocumentID  string
	WorkspaceID string
	GraphID     string
}

type Notifier interface {
	Queued(ctx context.Context, payload Payload)
	Running(ctx context.Context, payload Payload)
	Stage(ctx context.Context, payload Payload, stage string)
	Failed(ctx context.Context, payload Payload, errorMessage string)
	Completed(ctx context.Context, payload Payload)
}

type noopNotifier struct{}

func (noopNotifier) Queued(context.Context, Payload)         {}
func (noopNotifier) Running(context.Context, Payload)        {}
func (noopNotifier) Stage(context.Context, Payload, string)  {}
func (noopNotifier) Failed(context.Context, Payload, string) {}
func (noopNotifier) Completed(context.Context, Payload)      {}

type firestoreNotifier struct {
	client *firestore.Client
}

func NewNotifier(ctx context.Context, projectID string) Notifier {
	if projectID == "" {
		return noopNotifier{}
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		log.Printf("jobstatus: firebase app init failed: %v", err)
		return noopNotifier{}
	}
	client, err := app.Firestore(ctx)
	if err != nil {
		log.Printf("jobstatus: firestore init failed: %v", err)
		return noopNotifier{}
	}
	return &firestoreNotifier{client: client}
}

func (n *firestoreNotifier) Queued(ctx context.Context, payload Payload) {
	n.write(ctx, payload, map[string]any{
		"status":       "queued",
		"currentStage": "",
		"errorMessage": "",
		"createdAt":    nowRFC3339(),
		"updatedAt":    nowRFC3339(),
	})
}

func (n *firestoreNotifier) Running(ctx context.Context, payload Payload) {
	n.write(ctx, payload, map[string]any{
		"status":       "running",
		"errorMessage": "",
		"updatedAt":    nowRFC3339(),
	})
}

func (n *firestoreNotifier) Stage(ctx context.Context, payload Payload, stage string) {
	n.write(ctx, payload, map[string]any{
		"status":       "running",
		"currentStage": stage,
		"updatedAt":    nowRFC3339(),
	})
}

func (n *firestoreNotifier) Failed(ctx context.Context, payload Payload, errorMessage string) {
	n.write(ctx, payload, map[string]any{
		"status":       "failed",
		"currentStage": "",
		"errorMessage": errorMessage,
		"updatedAt":    nowRFC3339(),
	})
}

func (n *firestoreNotifier) Completed(ctx context.Context, payload Payload) {
	n.write(ctx, payload, map[string]any{
		"status":       "completed",
		"currentStage": "",
		"errorMessage": "",
		"updatedAt":    nowRFC3339(),
	})
}

func (n *firestoreNotifier) write(ctx context.Context, payload Payload, fields map[string]any) {
	doc := map[string]any{
		"jobId":       payload.JobID,
		"jobType":     payload.JobType,
		"documentId":  payload.DocumentID,
		"workspaceId": payload.WorkspaceID,
		"graphId":     payload.GraphID,
	}
	for key, value := range fields {
		doc[key] = value
	}
	_, err := n.client.Collection("workspaces").Doc(payload.WorkspaceID).Collection("jobs").Doc(payload.JobID).Set(ctx, doc, firestore.MergeAll)
	if err != nil {
		log.Printf("jobstatus: firestore write failed: %v", err)
	}
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
